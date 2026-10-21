package shapes_test

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/shapes"
	"botropolis.city/botropolis/pkg/xvfb"
)

// The painter draws through the GPU, so these tests run inside an
// Ebitengine game on a private virtual display, the way Ebitengine's own
// tests do: the game's first Update runs them, then ends the game.
type game struct {
	m    *testing.M
	code int
}

func (g *game) Update() error {
	g.code = g.m.Run()
	return ebiten.Termination
}

func (*game) Draw(*ebiten.Image)         {}
func (*game) Layout(int, int) (int, int) { return 64, 64 }
func (g *game) run() error               { return ebiten.RunGame(g) }
func setenv(kv string) error             { k, v, _ := strings.Cut(kv, "="); return os.Setenv(k, v) }
func fail(format string, args ...any) (code int) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

func TestMain(m *testing.M) {
	os.Exit(main(m))
}

func main(m *testing.M) int {
	if _, err := exec.LookPath("Xvfb"); err != nil {
		if os.Getenv("CI") != "" {
			return fail("Xvfb is not installed, and CI must test the painter on a virtual display")
		}
		fmt.Fprintln(os.Stderr, "Xvfb is not installed, so the painter went untested")
		return 0
	}
	dir, err := os.MkdirTemp("", "shapes-xvfb-")
	if err != nil {
		return fail("making a directory for Xvfb: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	s, err := xvfb.Start(dir, 320, 240)
	if err != nil {
		return fail("starting Xvfb: %v", err)
	}
	defer func() { _ = s.Close() }()
	for _, kv := range s.Env() {
		if err := setenv(kv); err != nil {
			return fail("setting %s: %v", kv, err)
		}
	}
	ebiten.SetWindowSize(64, 64)
	g := &game{m: m, code: 1}
	if err := g.run(); err != nil {
		return fail("running the game: %v", err)
	}
	return g.code
}

var (
	paper = color.NRGBA{R: 10, G: 10, B: 10, A: 255}
	lot   = color.NRGBA{R: 200, G: 120, B: 60, A: 255}
)

func canvas(t *testing.T) *ebiten.Image {
	t.Helper()
	img := ebiten.NewImage(64, 64)
	t.Cleanup(img.Deallocate)
	img.Fill(paper)
	return img
}

func rgba(img *ebiten.Image, x, y int) color.RGBA {
	return color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
}

func far(a color.RGBA, b color.NRGBA) int {
	d := 0
	for _, v := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		d = max(d, int(math.Abs(float64(v[0])-float64(v[1]))))
	}
	return d
}

func rect(x0, y0, x1, y1 float64) []city.Point {
	return []city.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

func TestSeams(t *testing.T) {
	for _, edge := range []float64{32, 32.25, 32.5, 32.75} {
		t.Run(fmt.Sprintf("when two same-colour squares share an edge at x=%v", edge), func(t *testing.T) {
			img := canvas(t)
			var p shapes.Painter
			p.Fill(img, rect(10.4, 10.4, edge, 50.6), lot)
			p.Fill(img, rect(edge, 10.4, 53.6, 50.6), lot)

			t.Run("it should leave the shared edge's pixels within 3 levels of the fill", func(t *testing.T) {
				worst := 0
				for y := 14; y < 47; y++ {
					for x := int(edge) - 2; x <= int(edge)+2; x++ {
						worst = max(worst, far(rgba(img, x, y), lot))
					}
				}
				assert.LessOrEqual(t, worst, 3)
			})
		})
	}

	t.Run("when two same-colour squares share a slanted edge", func(t *testing.T) {
		img := canvas(t)
		var p shapes.Painter
		p.Fill(img, []city.Point{{X: 8, Y: 8}, {X: 30, Y: 8}, {X: 36.3, Y: 56}, {X: 8, Y: 56}}, lot)
		p.Fill(img, []city.Point{{X: 30, Y: 8}, {X: 56, Y: 8}, {X: 56, Y: 56}, {X: 36.3, Y: 56}}, lot)

		t.Run("it should leave the shared edge's pixels within 3 levels of the fill", func(t *testing.T) {
			worst := 0
			for y := 12; y < 52; y++ {
				mid := 30 + (36.3-30)*(float64(y)+0.5-8)/48
				for x := int(mid) - 2; x <= int(mid)+2; x++ {
					worst = max(worst, far(rgba(img, x, y), lot))
				}
			}
			assert.LessOrEqual(t, worst, 3)
		})
	})
}

func TestFallback(t *testing.T) {
	t.Run("when a ring crosses itself", func(t *testing.T) {
		img := canvas(t)
		var p shapes.Painter
		p.Fill(img, []city.Point{{X: 8, Y: 8}, {X: 56, Y: 56}, {X: 56, Y: 8}, {X: 8, Y: 56}}, lot)

		t.Run("it should still paint each of its lobes", func(t *testing.T) {
			assert.Equal(t, []int{0, 0}, []int{far(rgba(img, 50, 32), lot), far(rgba(img, 13, 32), lot)})
		})
	})
}

func TestDisc(t *testing.T) {
	for _, r := range []float64{1.5, 4, 9.7, 25} {
		t.Run(fmt.Sprintf("when a disc of radius %v is drawn", r), func(t *testing.T) {
			img := canvas(t)
			var p shapes.Painter
			p.Disc(img, city.Point{X: 32, Y: 32}, r, lot)

			t.Run("it should paint its centre the colour", func(t *testing.T) {
				assert.LessOrEqual(t, far(rgba(img, 31, 31), lot), 3)
			})

			t.Run("it should leave a pixel two beyond its edge clear", func(t *testing.T) {
				x := int(32 + r + 2)
				assert.Zero(t, far(rgba(img, x, 32), paper))
			})
		})
	}
}

func TestLine(t *testing.T) {
	t.Run("when a line is stroked three pixels wide", func(t *testing.T) {
		img := canvas(t)
		var p shapes.Painter
		p.Line(img, 8, 32, 56, 32, 3, lot)

		t.Run("it should paint the pixels on it the colour", func(t *testing.T) {
			assert.LessOrEqual(t, far(rgba(img, 30, 32), lot), 3)
		})

		t.Run("it should leave a pixel three away clear", func(t *testing.T) {
			assert.Zero(t, far(rgba(img, 30, 35), paper))
		})
	})
}
