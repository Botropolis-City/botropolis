package render

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// frameCheckSize is the window the frame check draws at: the size item
// 194's frame test measures.
var frameCheckSize = image.Pt(1600, 900)

// frameVariant is one way the city is shown while the check runs.
type frameVariant struct {
	name string
	set  func(d *Drive)
}

func frameVariants() []frameVariant {
	return []frameVariant{
		{"as it stands", func(*Drive) {}},
		// Selecting changes one still building and nothing the layer's
		// key holds, so only what the stills would paint can tell.
		{"with a building selected", func(d *Drive) {
			if bs := d.game.scene.City().Buildings(); len(bs) > 0 {
				d.game.scene.Select(bs[len(bs)/2].Session.ID)
			}
		}},
		{"with traffic shown and a car hovered", func(d *Drive) {
			g := d.game
			g.scene.SetView(city.ViewTraffic)
			cars := g.cars(g.scene.City(), g.motion.ambient)
			if len(cars) > 0 {
				_ = d.PointAtPoint(g.scene.Camera().WorldToScreen(cars[0].at))
			}
		}},
		{"with a view up", func(d *Drive) {
			_ = d.PointAtPoint(city.Point{X: -100, Y: -100})
			d.game.scene.SetView(city.ViewSpend)
		}},
	}
}

// CheckFrames draws the city staged in home, as it is at a moment, for a
// number of frames in each of the check's variants, each frame twice:
// straight, as every frame was drawn before item 193, and through the
// standing city composed once and drawn again where something moves. It
// reports every frame whose two drawings differ by more than
// lookTolerance, and writes both to dump when one is given.
func CheckFrames(home string, at time.Time, frames int, dump string) ([]string, error) {
	var problems []string
	err := runSheet(func(*Game) error {
		d, err := NewDrive(DriveOptions{Home: home, Now: at, Width: frameCheckSize.X, Height: frameCheckSize.Y})
		if err != nil {
			return err
		}
		g := d.game
		straight := ebiten.NewImageWithOptions(image.Rectangle{Max: frameCheckSize}, &ebiten.NewImageOptions{Unmanaged: true})
		composed := ebiten.NewImageWithOptions(image.Rectangle{Max: frameCheckSize}, &ebiten.NewImageOptions{Unmanaged: true})
		defer straight.Deallocate()
		defer composed.Deallocate()
		for _, v := range frameVariants() {
			v.set(d)
			for i := range frames {
				if err := g.Update(); err != nil {
					return err
				}
				// Both drawings are one frame to the look cache, so each
				// draws a building from its look or straight alike (item
				// 192).
				frame := g.lookFrame
				want := g.worldFrame(straight, true)
				g.lookFrame = frame
				got := g.worldFrame(composed, false)
				if off, box, most := framesApart(want, got); off > 0 {
					problems = append(problems, fmt.Sprintf("%s, frame %d: %d pixels in %v differ from the city drawn straight, by up to %d", v.name, i, off, box, most))
					if dump != "" {
						dumpPNG(filepath.Join(dump, fmt.Sprintf("%s-%02d-straight.png", v.name, i)), want)
						dumpPNG(filepath.Join(dump, fmt.Sprintf("%s-%02d-composed.png", v.name, i)), got)
					}
				}
			}
		}
		return nil
	})
	return problems, err
}

// worldFrame is one frame of the world as drawIso draws it, onto img
// filled first with a colour nothing in the city is, read back.
func (g *Game) worldFrame(img *ebiten.Image, straight bool) *image.NRGBA {
	img.Fill(colorCheckFill)
	g.noStill = straight
	defer func() { g.noStill = false }()
	g.lit = g.scene.Highlight()
	b := img.Bounds()
	g.drawIso(img, g.scene.City(), g.scene.Camera(), g.scene.Hover(), g.scene.Selected(), float64(b.Dx()), float64(b.Dy()), g.clock())
	out := image.NewNRGBA(b)
	img.ReadPixels(out.Pix)
	return out
}

// colorCheckFill is what the frame check fills a frame with first, so a
// pixel the city leaves undrawn shows.
var colorCheckFill = color.NRGBA{R: 255, B: 255, A: 255}

func dumpPNG(path string, img *image.NRGBA) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		dump(path, img)
	}
}
