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
	"botropolis.city/botropolis/pkg/state"
)

// frameCheckSize is the window the frame check draws at: the size item
// 194's frame test measures.
var frameCheckSize = image.Pt(1600, 900)

// frameVariant is one way the city is shown while the check runs.
type frameVariant struct {
	name, slug string
	set        func(d *Drive)
	// each is done before every frame of the way, when it is set.
	each func(d *Drive)
}

func frameVariants() []frameVariant {
	return []frameVariant{
		{"as it stands", "as-it-stands", func(*Drive) {}, nil},
		// Selecting changes one still building and nothing the layer's
		// key holds, so only what the stills would paint can tell.
		{"with a building selected", "selected", func(d *Drive) {
			if bs := d.game.scene.City().Buildings(); len(bs) > 0 {
				d.game.scene.Select(bs[len(bs)/2].Session.ID)
			}
		}, nil},
		{"with traffic shown and a car hovered", "traffic-hovered", func(d *Drive) {
			g := d.game
			g.scene.SetView(city.ViewTraffic)
			cars := g.cars(g.scene.City(), g.motion.ambient)
			if len(cars) > 0 {
				_ = d.PointAtPoint(g.scene.Camera().WorldToScreen(cars[0].at))
			}
		}, nil},
		{"with a view up", "spend-view", func(d *Drive) {
			_ = d.PointAtPoint(city.Point{X: -100, Y: -100})
			d.game.scene.SetView(city.ViewSpend)
		}, nil},
		// Item 193: a pan by whole pixels shifts the standing city and
		// composes the strips it uncovers; by part of a pixel it cannot,
		// and the city is drawn straight until the camera rests.
		{"zoomed in and panned by whole pixels", "pan-whole", func(d *Drive) {
			d.game.scene.SetView(city.ViewAttention)
			d.game.scene.Wheel(city.Point{X: float64(frameCheckSize.X) / 2, Y: float64(frameCheckSize.Y) / 2}, 2)
			// Let the zoom settle, so the pan is the only thing moving.
			_ = d.Run(120)
		}, func(d *Drive) {
			d.game.scene.Pan(city.Point{X: -3, Y: 2})
		}},
		{"panned by part of a pixel", "pan-part", nil, func(d *Drive) {
			d.game.scene.Pan(city.Point{X: 0.5, Y: -0.25})
		}},
	}
}

// CheckFrames draws the city staged in home, as it is at a moment, for a
// number of frames in each of the check's variants, each frame twice:
// straight, as every frame was drawn before item 193, and through the
// standing city composed once and drawn again where something moves. It
// reports every frame whose two drawings differ by more than
// lookTolerance, and writes both to dump when one is given. With write,
// it also writes both drawings of each way's last frame there, for a
// person to judge.
func CheckFrames(home string, at time.Time, frames int, dump, write string) ([]string, error) {
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
			if v.set != nil {
				v.set(d)
			}
			for i := range frames {
				if v.each != nil {
					v.each(d)
				}
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
				if write != "" && i == frames-1 {
					dumpPNG(filepath.Join(write, v.slug+"-straight.png"), want)
					dumpPNG(filepath.Join(write, v.slug+"-standing.png"), got)
				}
				if off, box, most := framesApart(want, got); off > 0 {
					problems = append(problems, fmt.Sprintf("%s, frame %d: %d pixels in %v differ from the city drawn straight, by up to %d", v.name, i, off, box, most))
					if dump != "" {
						dumpPNG(filepath.Join(dump, fmt.Sprintf("%s-%02d-straight.png", v.name, i)), want)
						dumpPNG(filepath.Join(dump, fmt.Sprintf("%s-%02d-composed.png", v.name, i)), got)
					}
				}
			}
		}
		more, err := g.checkPanels(straight, composed, dump, write)
		problems = append(problems, more...)
		return err
	})
	return problems, err
}

// checkPanels opens each panel that is baked while it holds and draws
// it over the same image straight and then through its bake three
// times, the frames it is played straight, baked and blitted on, and
// reports each that differs from it drawn straight by more than
// lookTolerance (item 161).
func (g *Game) checkPanels(straight, composed *ebiten.Image, dump, write string) ([]string, error) {
	var problems []string
	for _, p := range []struct {
		name, slug string
		open       func() bool
	}{
		{"the journal", "journal", func() bool {
			g.openJournal()
			_, ok := g.overlays.top().(*journal)
			return ok
		}},
		{"the group review", "group-review", func() bool {
			snap := g.scene.Snapshot()
			if len(snap.Sessions) < 2 {
				return false
			}
			ids := []string{snap.Sessions[0].ID, snap.Sessions[1].ID}
			snap.Groups = []state.Group{{ID: "check", Name: "CHECK-1 · panels", Short: "CHECK-1", Reason: "the frame check", Members: ids}}
			snap.GroupsReviewed = false
			g.scene.SetSnapshot(snap)
			return g.openFirstRun()
		}},
	} {
		g.overlays = g.overlays[:0]
		if !p.open() {
			problems = append(problems, p.name+" did not open")
			continue
		}
		want := g.panelFrame(straight, true)
		for i, way := range []string{"played straight", "baked", "blitted"} {
			got := g.panelFrame(composed, false)
			if i == 2 && g.panels.stats.blits == 0 {
				problems = append(problems, p.name+" was never blitted from its bake")
			}
			if write != "" && i == 2 {
				dumpPNG(filepath.Join(write, "panel-"+p.slug+"-straight.png"), want)
				dumpPNG(filepath.Join(write, "panel-"+p.slug+"-baked.png"), got)
			}
			if off, box, most := framesApart(want, got); off > 0 {
				problems = append(problems, fmt.Sprintf("%s, %s: %d pixels in %v differ from it drawn straight, by up to %d", p.name, way, off, box, most))
				if dump != "" {
					dumpPNG(filepath.Join(dump, fmt.Sprintf("panel-%s-%d-straight.png", p.slug, i)), want)
					dumpPNG(filepath.Join(dump, fmt.Sprintf("panel-%s-%d-baked.png", p.slug, i)), got)
				}
			}
		}
	}
	g.overlays = g.overlays[:0]
	return problems, nil
}

// panelFrame is the open panels drawn onto img, filled first with a
// colour nothing is, straight or through their bakes, read back.
func (g *Game) panelFrame(img *ebiten.Image, straight bool) *image.NRGBA {
	img.Fill(colorCheckFill)
	g.panels.straight = straight
	defer func() { g.panels.straight = false }()
	g.panels.stats = panelStats{}
	b := img.Bounds()
	g.drawOverlays(img, float64(b.Dx()), float64(b.Dy()), false)
	out := image.NewNRGBA(b)
	img.ReadPixels(out.Pix)
	return out
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
