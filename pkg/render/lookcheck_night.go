package render

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// nightPair is a lit tower with a building standing in front of it, as
// the night check draws them.
type nightPair struct {
	back, front *city.Building
	cams        [2]*city.Camera
}

func (g *Game) nightPair(zoom float64, heading int, frontLit bool) nightPair {
	back := sheetBuilding(city.ResidentialDistrict, state.MaxStoreys, false)
	front := dressedBuilding()
	front.Lit = frontLit
	front.Session.ID = "sheet-front"
	// On whole pixels: seenThroughFront takes a pixel two in from the
	// front building's edge as solid, and off the grid its filtered edge
	// reaches a pixel further at a level or so, so a soft edge pixel would
	// count as solid. The day check holds the cache to a sub-pixel phase.
	foot := city.Point{X: math.Floor(lookFoot.X), Y: math.Floor(lookFoot.Y)}
	feet := [2]city.Point{foot, {X: foot.X + 10*zoom, Y: foot.Y + 40*zoom}}
	var cams [2]*city.Camera
	for i, b := range []*city.Building{back, front} {
		cam := &city.Camera{Zoom: zoom, Heading: heading}
		at := cam.Project(b.Rect.Center())
		cam.Offset = city.Point{X: feet[i].X/zoom - at.X, Y: feet[i].Y/zoom - at.Y}
		cams[i] = cam
	}
	return nightPair{back: back, front: front, cams: cams}
}

// nightFrame draws a pair at night as drawIso does: the buildings back to
// front, the wash, then the lights. A frame through the cache is counted
// as a frame of it.
func (g *Game) nightFrame(p nightPair, cached, lit bool) *image.NRGBA {
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, lookCellSize, lookCellSize), &ebiten.NewImageOptions{Unmanaged: true})
	defer img.Deallocate()
	img.Fill(colorVoid)
	g.noLookCache = !cached
	if cached {
		g.nextLookFrame()
	}
	back := *p.back
	back.Lit = p.back.Lit && lit
	g.beginLights(lookCellSize, lookCellSize)
	g.drawBuilding(img, p.cams[0], &back, false, true, 0)
	g.drawBuilding(img, p.cams[1], p.front, false, true, 0)
	wash(img, 0, 0, lookCellSize, lookCellSize)
	g.endLights(img)
	g.noLookCache = false
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// alone is a building drawn on its own at night, directly, or the night
// with no building in it.
func (g *Game) alone(cam *city.Camera, b *city.Building) *image.NRGBA {
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, lookCellSize, lookCellSize), &ebiten.NewImageOptions{Unmanaged: true})
	defer img.Deallocate()
	img.Fill(colorVoid)
	if b != nil {
		g.noLookCache = true
		g.drawBuilding(img, cam, b, false, true, 0)
		g.noLookCache = false
	}
	wash(img, 0, 0, lookCellSize, lookCellSize)
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// afterLights is where a building paints after its own lights are noted:
// its sign, beacon, flags, scaffold and spire. Drawn straight, each cuts
// the lights as item 195 does, a line as a hard bar a pixel wider than
// it; baked, the whole look cuts them at its own alpha. Both hide the
// light behind them, a pixel of edge apart, so the night check does not
// hold those pixels to the frame drawn straight.
func (g *Game) afterLights(cam *city.Camera, b *city.Building) *image.NRGBA {
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, lookCellSize, lookCellSize), &ebiten.NewImageOptions{Unmanaged: true})
	defer img.Deallocate()
	out := image.NewNRGBA(img.Bounds())
	f, ok := g.buildingFrame(cam, b, false, true, 0)
	if !ok {
		return out
	}
	g.drawSteps(img, cam, &f, stepBeacon, stepCount)
	img.ReadPixels(out.Pix)
	return out
}

// checkLooksAtNight draws a lit tower behind a building, through the
// cache and directly, and reports a lit window seen through the cached
// building, or a cached night frame that differs from the direct one
// anywhere but where a building paints after its own lights.
func (g *Game) checkLooksAtNight(zoom float64, heading int) []string {
	name := fmt.Sprintf("a lit tower behind a cached building at night, heading %d", heading)
	var problems []string

	dark := g.nightPair(zoom, heading, false)
	g.looks, g.lookFrame = nil, 0
	for range 2 {
		g.nightFrame(dark, true, true)
	}
	on, off := g.nightFrame(dark, true, true), g.nightFrame(dark, true, false)
	mask, blank := g.alone(dark.cams[1], dark.front), g.alone(nil, nil)
	if dir := os.Getenv("BOTROPOLIS_LOOK_DUMP"); dir != "" {
		for n, img := range map[string]*image.NRGBA{"on": on, "off": off, "mask": mask, "blank": blank} {
			dump(fmt.Sprintf("%s/night-%d-%s.png", dir, heading, n), img)
		}
	}
	if g.lookStats.hits == 0 {
		problems = append(problems, name+": the building in front was not drawn from the cache")
	}
	problems = append(problems, seenThroughFront(name, on, off, mask, blank, g.solidMargin(zoom))...)

	lit := g.nightPair(zoom, heading, true)
	g.looks, g.lookFrame = nil, 0
	for range 2 {
		g.nightFrame(lit, true, true)
	}
	got, want := g.nightFrame(lit, true, true), g.nightFrame(lit, false, true)
	skip := []*image.NRGBA{g.afterLights(lit.cams[0], lit.back), g.afterLights(lit.cams[1], lit.front)}
	if off, box, most := framesApartExcept(want, got, skip); off > 0 {
		problems = append(problems, fmt.Sprintf("%s, both lit: %d pixels in %v differ from the frame drawn directly, by up to %d", name, off, box, most))
	}
	return problems
}

// framesApartExcept is framesApart over the pixels no skip image paints,
// nor any pixel beside one.
func framesApartExcept(a, b *image.NRGBA, skip []*image.NRGBA) (int, image.Rectangle, int) {
	painted := func(x, y int) bool {
		for _, s := range skip {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if s.NRGBAAt(x+dx, y+dy).A > 0 {
						return true
					}
				}
			}
		}
		return false
	}
	ka, kb := image.NewNRGBA(a.Bounds()), image.NewNRGBA(b.Bounds())
	copy(ka.Pix, a.Pix)
	copy(kb.Pix, b.Pix)
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if painted(x, y) {
				kb.SetNRGBA(x, y, ka.NRGBAAt(x, y))
			}
		}
	}
	return framesApart(ka, kb)
}
