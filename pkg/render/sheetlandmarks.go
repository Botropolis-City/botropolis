package render

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// landmarkCellSize is the picture each landmark is checked in: City Hall
// stands taller than a building's cell at zoom 2.
const landmarkCellSize = 768

// checkLandmarks draws each landmark at every heading at night, its
// lights on and off, and reports one that lights a pixel off its panes,
// lights nothing though it has panes, or lights its own painted name;
// then a lit landmark behind a dark building, reporting any light that
// shows through the building in front. Item 185.
func (g *Game) checkLandmarks(zoom float64) []string {
	var problems []string
	names := 0
	for _, kit := range []string{kitPlant, kitLibrary, kitHall} {
		for _, heading := range assets.Headings {
			cam, lot := landmarkSheetCam(zoom, heading, kit, city.Point{X: landmarkCellSize / 2, Y: landmarkCellSize - 32})
			on, off := g.sheetLandmark(zoom, cam, lot, kit, 1, nil, city.Point{}), g.sheetLandmark(zoom, cam, lot, kit, 0, nil, city.Point{})
			day := g.sheetLandmarkByDay(cam, lot, kit)
			panes := g.landmarkPanes(cam, lot, kit)
			name, lettered := g.landmarkLettering(cam, cityLandmarks(landmarkCity(kit, lot)), 0)
			stray, lit, onName, warmer, warm := 0, 0, 0, 0, 0
			bounds := on.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					if on.NRGBAAt(x, y) == off.NRGBAAt(x, y) {
						continue
					}
					lit++
					warmer += warmth(on.NRGBAAt(x, y)) - warmth(day.NRGBAAt(x, y))
					warm += warmth(on.NRGBAAt(x, y))
					if !nearPane(panes, x, y, litMargin) {
						stray++
					}
					if lettered && inScreenQuad(name.Quad, city.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5}) {
						onName++
					}
				}
			}
			label := fmt.Sprintf("%s seen from %d", kit, heading)
			switch {
			case stray > 0:
				problems = append(problems, fmt.Sprintf("%s: %d lit pixels outside its panes", label, stray))
			case len(panes) > 0 && lit == 0:
				problems = append(problems, label+": has windows but lit nothing")
			case len(panes) == 0 && lit > 0:
				problems = append(problems, fmt.Sprintf("%s: has no windows but lit %d pixels", label, lit))
			}
			if lit > 0 && warmer < litWarmer*lit {
				problems = append(problems, fmt.Sprintf("%s: its lit glass is %.1f warmer than by day, on average, not %d", label, float64(warmer)/float64(lit), litWarmer))
			}
			if lit > 0 && warm < litWarm*lit {
				problems = append(problems, fmt.Sprintf("%s: its lit glass is %.1f redder than blue, on average, not %d: it reads grey or blue, not yellow", label, float64(warm)/float64(lit), litWarm))
			}
			if onName > 0 {
				problems = append(problems, fmt.Sprintf("%s: %d lit pixels on its painted name", label, onName))
			}
			if lettered {
				names++
			}
		}
	}
	if names == 0 {
		problems = append(problems, "no landmark's name was painted, so nothing checked the lights keep off it")
	}
	return append(problems, g.checkLandmarkOcclusion(zoom)...)
}

// checkLandmarkOcclusion draws City Hall lit with a dark tower standing
// in front of it, and reports any of its light that shows through the
// tower.
func (g *Game) checkLandmarkOcclusion(zoom float64) []string {
	back := city.Point{X: landmarkCellSize / 2, Y: landmarkCellSize * 0.6}
	cam, lot := landmarkSheetCam(zoom, 0, kitHall, back)
	front := sheetBuilding(city.OfficeDistrict, state.MaxStoreys, false)
	front.Lit = false
	front.Session.ID = "sheet-front"
	at := city.Point{X: back.X + 10*zoom, Y: back.Y + 60*zoom}
	on, off := g.sheetLandmark(zoom, cam, lot, kitHall, 1, front, at), g.sheetLandmark(zoom, cam, lot, kitHall, 0, front, at)
	mask := g.sheetLandmark(zoom, nil, lot, kitHall, 0, front, at)
	blank := g.sheetLandmark(zoom, nil, lot, kitHall, 0, nil, at)
	return seenThroughFront("a lit city hall behind a dark building", on, off, mask, blank)
}

// litWarmer is how much warmer, on average, a landmark's lit glass must
// be at night than by day: a little yellow (Aria, 2026-10-20).
const litWarmer = 20

// litWarm is how much redder than blue, on average, a landmark's lit
// glass must be at night, so it reads yellow: mixing the kit's blue
// toward yellow made it grey, which passed litWarmer and did not look
// yellow.
const litWarm = 15

// warmth is how much redder than blue a colour is.
func warmth(c color.NRGBA) int {
	return int(c.R) - int(c.B)
}

// sheetLandmarkByDay draws a landmark by day, as the map does: no wash
// and no lights.
func (g *Game) sheetLandmarkByDay(cam *city.Camera, lot city.Rect, kit string) *image.NRGBA {
	img := ebiten.NewImage(landmarkCellSize, landmarkCellSize)
	defer img.Deallocate()
	img.Fill(colorVoid)
	g.kit(img, cam, kit, landmarkTurn, lot.Center(), nil)
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// landmarkSheetCam is a camera that stands a landmark's lot on foot, and
// the lot.
func landmarkSheetCam(zoom float64, heading int, kit string, foot city.Point) (*city.Camera, city.Rect) {
	w, h := city.PlantWidth, city.PlantHeight
	switch kit {
	case kitLibrary:
		w, h = city.LibraryWidth, city.LibraryHeight
	case kitHall:
		w, h = city.HallWidth, city.HallHeight
	}
	lot := city.RectAt(0, 0, w, h)
	cam := &city.Camera{Zoom: zoom, Projection: city.Isometric, Heading: heading}
	at := cam.Project(lot.Center())
	cam.Offset = city.Point{X: foot.X/zoom - at.X, Y: foot.Y/zoom - at.Y}
	return cam, lot
}

// landmarkCity is a city holding only one landmark, on lot.
func landmarkCity(kit string, lot city.Rect) *city.City {
	c := &city.City{}
	switch kit {
	case kitPlant:
		c.Plant.Rect = lot
	case kitLibrary:
		c.Library.Rect = lot
	case kitHall:
		c.Hall.Rect = lot
	}
	return c
}

// landmarkPanes is a landmark's panes where it stands on its lot.
func (g *Game) landmarkPanes(cam *city.Camera, lot city.Rect, kit string) [][4]city.Point {
	atlas := g.kits.pick(cam.Zoom)
	sprite, ok := atlas.Sprite(kit, cam.Heading-landmarkTurn)
	_, origin, scale, _, placed := g.kitPlace(cam, kit, landmarkTurn, lot.Center(), 0)
	if !ok || !placed {
		return nil
	}
	out := make([][4]city.Point, 0, len(sprite.Panes))
	for _, p := range sprite.Panes {
		var q [4]city.Point
		for i, c := range p {
			q[i] = city.Point{X: origin.X + float64(c.X)*scale, Y: origin.Y + float64(c.Y)*scale}
		}
		out = append(out, q)
	}
	return out
}

// sheetLandmark draws a landmark at night as the map does — the sprite,
// its painted name, its lights noted, then anything in front, the wash
// and the lights — with its lights on as far as on, and, when front is
// not nil, a building standing in front of it on foot. A nil camera
// leaves the landmark out.
func (g *Game) sheetLandmark(zoom float64, cam *city.Camera, lot city.Rect, kit string, on float64, front *city.Building, foot city.Point) *image.NRGBA {
	img := ebiten.NewImage(landmarkCellSize, landmarkCellSize)
	defer img.Deallocate()
	img.Fill(colorVoid)
	g.beginLights(landmarkCellSize, landmarkCellSize)
	if cam != nil {
		g.kit(img, cam, kit, landmarkTurn, lot.Center(), nil)
		g.letterLandmark(img, cam, landmarkCity(kit, lot), kit)
		g.lightLandmark(cam, lot, kit, on)
	}
	if front != nil {
		fc := &city.Camera{Zoom: zoom}
		p := fc.Project(front.Rect.Center())
		fc.Offset = city.Point{X: foot.X/zoom - p.X, Y: foot.Y/zoom - p.Y}
		g.kitRising(img, fc, buildingPiece(front), g.footOf(fc, front), nil, 1)
	}
	vector.FillRect(img, 0, 0, landmarkCellSize, landmarkCellSize, colorNightOverlay, false)
	g.endLights(img)
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}
