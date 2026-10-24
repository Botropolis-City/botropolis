package render

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// lookTolerance is how far, in any channel, a blitted look may be from
// the building drawn directly: a soft edge composited into a baked image
// and then onto the frame rounds to 8 bits twice, and a shape drawn into
// an atlas-packed image rounds a level apart from one drawn on a texture
// of its own, as item 191 measured (within 3).
const lookTolerance = 3

// lookCellSize is the side of the frame the check draws each look on:
// big enough to hold the tallest tower's look box at the closest zoom,
// since a look is baked exactly only where it lies wholly on the frame
// (item 233).
const lookCellSize = 1024

// lookFoot is where the check stands each building: off the pixel grid,
// so the bake has a sub-pixel phase to keep.
var lookFoot = city.Point{X: lookCellSize/2 + 0.3, Y: lookCellSize/2 + 135.3}

// checkLook is one way a building can look, and how it changes after it
// was baked, for the check that the cache serves the new look.
type checkLook struct {
	name     string
	building func() *city.Building
	selected bool
	change   func(*city.Building)
}

func checkLooks() []checkLook {
	var out []checkLook
	for _, k := range sheetKinds {
		for tier := 0; tier <= state.MaxStoreys; tier++ {
			for _, boarded := range []bool{false, true} {
				kind, tier, boarded := k.kind, tier, boarded
				out = append(out, checkLook{
					name:     fmt.Sprintf("%s tier %d boarded %v", k.name, tier, boarded),
					building: func() *city.Building { return sheetBuilding(kind, tier, boarded) },
					change:   func(b *city.Building) { b.Session.Spire = 2 },
				})
			}
		}
	}
	dressed := dressedBuilding
	out = append(out,
		checkLook{name: "dressed: sign, spire, courses, flags, scaffold", building: dressed,
			change: func(b *city.Building) { b.Session.Title = "ship the release" }},
		checkLook{name: "dressed and selected", building: dressed, selected: true,
			change: func(b *city.Building) { b.Flags = 1 }},
		checkLook{name: "dressed, its compactions past its courses", building: func() *city.Building {
			b := dressed()
			b.Session.Compactions = maxCourses + 3
			return b
		}, change: func(b *city.Building) { b.Session.Compactions = maxCourses + 4 }},
		checkLook{name: "unattended", building: func() *city.Building {
			b := dressed()
			b.Session.State = state.Unattended
			return b
		}, change: func(b *city.Building) { b.Session.State = state.Working }},
		checkLook{name: "behind its blinds", building: func() *city.Building {
			b := dressed()
			b.Blinds = true
			return b
		}, change: func(b *city.Building) { b.Blinds = false }},
	)
	return out
}

// dressedBuilding is a tower with every dressing a still building can
// carry: a sign, a spire, courses, flags and a scaffold.
func dressedBuilding() *city.Building {
	b := sheetBuilding(city.OfficeDistrict, state.MaxStoreys, false)
	b.Session.Title = "fix the flaky build"
	b.Session.Spire = 3
	b.Session.Compactions = 2
	b.Flags = 2
	b.Merged = 1
	b.Session.PRs = []claude.PR{{Number: 1}, {Number: 2, State: claude.PRClosed}}
	return b
}

// CheckLooks draws every building the sheet does, and dressed ones, from
// each heading through the look cache and straight onto a frame, and
// reports each whose cached frame differs from its direct one: as it is
// first seen, baked, kept, and once its look changes. Item 192.
func CheckLooks(zoom float64) ([]string, error) {
	var problems []string
	err := runSheet(func(g *Game) error {
		for _, look := range checkLooks() {
			for _, heading := range []int{0, 90, 180, 270} {
				problems = append(problems, g.checkLook(zoom, heading, look)...)
			}
		}
		for _, heading := range []int{0, 90, 180, 270} {
			problems = append(problems, g.checkLooksAtNight(zoom, heading)...)
		}
		return nil
	})
	return problems, err
}

func (g *Game) checkLook(zoom float64, heading int, look checkLook) []string {
	name := fmt.Sprintf("%s, heading %d", look.name, heading)
	b := look.building()
	cam := &city.Camera{Zoom: zoom, Heading: heading}
	at := cam.Project(b.Rect.Center())
	cam.Offset = city.Point{X: lookFoot.X/zoom - at.X, Y: lookFoot.Y/zoom - at.Y}
	g.looks, g.lookFrame = nil, 0
	var problems []string
	compare := func(when string, stats func(lookStats) string) {
		g.noLookCache = true
		want := g.lookFrameOf(cam, b, look.selected)
		g.noLookCache = false
		got := g.lookFrameOf(cam, b, look.selected)
		if off, box, most := framesApart(want, got); off > 0 {
			problems = append(problems, fmt.Sprintf("%s, %s: %d pixels in %v differ from the building drawn directly, by up to %d", name, when, off, box, most))
			if dir := os.Getenv("BOTROPOLIS_LOOK_DUMP"); dir != "" {
				dump(filepath.Join(dir, fmt.Sprintf("%s-%s-want.png", name, when)), want)
				dump(filepath.Join(dir, fmt.Sprintf("%s-%s-got.png", name, when)), got)
			}
		}
		if s := stats(g.lookStats); s != "" {
			problems = append(problems, fmt.Sprintf("%s, %s: %s", name, when, s))
		}
	}
	compare("first seen", func(s lookStats) string { return "" })
	compare("baked", func(s lookStats) string {
		if s.bakes == 0 {
			return "nothing was baked"
		}
		return ""
	})
	compare("kept", func(s lookStats) string {
		if s.hits == 0 || s.bakes > 0 {
			return fmt.Sprintf("not drawn from the cache: %d hits, %d bakes", s.hits, s.bakes)
		}
		return ""
	})
	look.change(b)
	compare("changed", func(s lookStats) string { return "" })
	return problems
}

// lookFrameOf is one frame of a building on its own, as drawIso draws it.
// Each frame draws twice, directly and through the cache, so only the
// second counts as the cache's frame.
func (g *Game) lookFrameOf(cam *city.Camera, b *city.Building, selected bool) *image.NRGBA {
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, lookCellSize, lookCellSize), &ebiten.NewImageOptions{Unmanaged: true})
	defer img.Deallocate()
	img.Fill(colorVoid)
	if !g.noLookCache {
		g.nextLookFrame()
	}
	g.drawBuilding(img, cam, b, selected, true, 0)
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// framesApart is how many pixels of two frames differ by more than
// lookTolerance in any channel, the box they are in, and the most any
// of them differs by.
func framesApart(a, b *image.NRGBA) (int, image.Rectangle, int) {
	off, most := 0, 0
	var box image.Rectangle
	for i := 0; i+3 < len(a.Pix); i += 4 {
		worst := 0
		for c := range 4 {
			worst = max(worst, int(a.Pix[i+c])-int(b.Pix[i+c]), int(b.Pix[i+c])-int(a.Pix[i+c]))
		}
		if worst <= lookTolerance {
			continue
		}
		off, most = off+1, max(most, worst)
		x, y := (i%a.Stride)/4, i/a.Stride
		box = box.Union(image.Rect(x, y, x+1, y+1))
	}
	return off, box, most
}

func dump(path string, img *image.NRGBA) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	_ = png.Encode(f, img)
	_ = f.Close()
}
