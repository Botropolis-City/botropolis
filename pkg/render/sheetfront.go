package render

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// inFront is something the map draws in front of a lit tower, by the
// map's own call: in the depth sort after the tower, or over the frame
// once the lights are laid (item 195).
type inFront struct {
	name string
	// sorted draws it in the depth sort, just after the tower, whose
	// camera, sprite box and foot it is given.
	sorted func(g *Game, img *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point)
	// over draws it over the frame after the lights.
	over func(g *Game, img *ebiten.Image, foot city.Point)
	// half is whether it is half transparent, and should leave half the
	// light behind it rather than none.
	half bool
	// placed is, for a thing that stands only where its words are
	// legible, whether the layout the drawing code uses places it in
	// front of the tower at this zoom (item 233).
	placed func(g *Game, cam *city.Camera, r city.Rect, foot city.Point) bool
}

// paintedTitle is the title painted on the tower in front of its lights.
const paintedTitle = "orchard works and the long lane"

// frontDistrict is the district whose sign stands in front of the tower.
func frontDistrict(cam *city.Camera, foot city.Point, zoom float64) *city.District {
	return signedDistrict(cam, city.Point{X: foot.X + 14*zoom, Y: foot.Y + 6*zoom})
}

// inFronts is every kind of thing the map draws in front of a lit
// building that is not a sprite.
func inFronts(zoom float64) []inFront {
	ink := color.NRGBA{R: 30, G: 30, B: 40, A: 255}
	half := color.NRGBA{R: 30, G: 30, B: 40, A: 128}
	return []inFront{
		{name: "a district sign", sorted: func(g *Game, img *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point) {
			g.districtSign(img, cam, frontDistrict(cam, foot, zoom))
		}, placed: func(g *Game, cam *city.Camera, r city.Rect, foot city.Point) bool {
			_, _, _, ok := g.districtSignLayout(cam, frontDistrict(cam, foot, zoom))
			return ok
		}},
		{name: "its own painted sign", sorted: func(g *Game, img *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point) {
			g.buildingSign(img, r, foot, paintedTitle)
		}, placed: func(g *Game, cam *city.Camera, r city.Rect, foot city.Point) bool {
			_, ok := g.buildingSignLayout(r, foot, paintedTitle)
			return ok
		}},
		{name: "painted shapes: a fill, a line and a disc", sorted: func(g *Game, img *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point) {
			mid := r.Center()
			g.fillRect(img, float32(r.Min.X), float32(mid.Y-8*zoom), float32(r.Width()), float32(10*zoom), ink)
			g.strokeLine(img, float32(r.Min.X), float32(r.Min.Y+r.Height()*0.2), float32(r.Max.X), float32(r.Min.Y+r.Height()*0.35), float32(4*zoom), ink)
			g.fillCircle(img, float32(mid.X), float32(r.Min.Y+r.Height()*0.7), float32(8*zoom), ink)
		}},
		{name: "a hard-edged box", sorted: func(g *Game, img *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point) {
			g.paint.Box(img, float32(r.Min.X), float32(r.Min.Y+r.Height()*0.4), float32(r.Width()), float32(12*zoom), ink)
		}},
		{name: "a half-transparent shape", half: true, sorted: func(g *Game, img *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point) {
			g.fillRect(img, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), half)
		}},
		{name: "a station label", over: func(g *Game, img *ebiten.Image, foot city.Point) {
			g.queueStationLabel(nil, city.Point{X: foot.X - 30*zoom, Y: foot.Y - 40*zoom}, 4*zoom, "millpond", colorText, g.theme.Palette.Panel)
		}},
		{name: "a badge", over: func(g *Game, img *ebiten.Image, foot city.Point) {
			b := sheetBuilding(city.ResidentialDistrict, state.MaxStoreys, false)
			b.Group = &state.Group{Name: "millpond", Short: "millpond"}
			g.queueBadge(b, city.Point{X: foot.X, Y: foot.Y - 60*zoom})
		}},
	}
}

// signedDistrict is a district whose sign stands at a screen point.
func signedDistrict(cam *city.Camera, at city.Point) *city.District {
	w := cam.ScreenToWorld(at)
	size := 4 * city.CellSize
	return &city.District{Name: "orchard", Rect: city.RectAt(w.X-3*city.CellSize, w.Y-3*city.CellSize, size, size)}
}

// Of a lit window's light, the share a pixel may show beyond what the
// thing in front leaves uncovered, for filtering; and how much a window
// must move a pixel for its share there to be read.
const (
	lightSlack = 0.15
	readable   = 48
)

// checkInFront draws a lit tower with each thing in front of it, and
// reports any pixel where more of a window's light shows than the thing
// in front leaves uncovered: drawn on its own over nothing, its alpha at
// a pixel is how much of the light behind it must go. So an opaque thing
// hides the light, a half-transparent one leaves half, and a word's soft
// edge leaves what its glyph does not cover.
func (g *Game) checkInFront(zoom float64) []string {
	var problems []string
	for _, f := range inFronts(zoom) {
		if !g.placedInFront(zoom, &f) {
			// Its own layout leaves it off below ui.MinSignPx; left off
			// even at the closest zoom, it would never be drawn.
			if zoom >= city.MaxZoom {
				problems = append(problems, fmt.Sprintf("a lit tower behind %s: its layout leaves it off even at zoom %g, the closest", f.name, zoom))
			} else {
				g.checkNotes = append(g.checkNotes, fmt.Sprintf("%s is not drawn at zoom %g: its layout leaves it off below %g px", f.name, zoom, ui.MinSignPx))
			}
			continue
		}
		on, off := g.frontScene(zoom, true, &f), g.frontScene(zoom, false, &f)
		bare, bareOff := g.frontScene(zoom, true, nil), g.frontScene(zoom, false, nil)
		cover := g.frontAlone(zoom, &f)
		name := "a lit tower behind " + f.name
		if f.over != nil {
			name = "a lit tower under " + f.name
		}
		drawn, read, through := 0, 0, 0
		var left float64
		bounds := on.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				a := float64(cover.NRGBAAt(x, y).A) / 255
				if a == 0 {
					continue
				}
				drawn++
				if brighter(bare.NRGBAAt(x, y), bareOff.NRGBAAt(x, y)) < readable {
					continue
				}
				// A thing in the depth sort has the lights cut by it, and
				// is measured against them as cut; one laid over the
				// frame covers the light whole, so its share is of the
				// light itself. Against the cut lights, a pale label
				// left a denominator of a few levels, and a level of
				// rounding read as half the light (item 233).
				full := brighter(bare.NRGBAAt(x, y), off.NRGBAAt(x, y))
				if f.over != nil {
					full = brighter(bare.NRGBAAt(x, y), bareOff.NRGBAAt(x, y))
				}
				if full <= 0 {
					continue
				}
				read++
				share := float64(max(brighter(on.NRGBAAt(x, y), off.NRGBAAt(x, y)), 0)) / float64(full)
				left += share
				if share > 1-a+lightSlack {
					through++
				}
			}
		}
		switch {
		case drawn == 0:
			problems = append(problems, name+": drew nothing in front")
		case read == 0:
			problems = append(problems, name+": covers no lit window")
		case through > 0:
			problems = append(problems, fmt.Sprintf("%s: %d pixels show more of the light behind than it leaves uncovered", name, through))
		}
		if f.half && read > 0 {
			if mean := left / float64(read); mean < 0.4 || mean > 0.6 {
				problems = append(problems, fmt.Sprintf("%s: left %.0f%% of the light behind it, not half", name, mean*100))
			}
		}
	}
	return problems
}

// sheetTower is the lit tower things stand in front of, and its camera,
// in a check's picture.
func sheetTower(zoom float64, lit bool) (*city.Building, *city.Camera) {
	b := sheetBuilding(city.ResidentialDistrict, state.MaxStoreys, false)
	b.Lit = lit
	foot := city.Point{X: sheetCellSize / 2, Y: sheetCellSize * 0.75}
	cam := &city.Camera{Zoom: zoom}
	at := cam.Project(b.Rect.Center())
	cam.Offset = city.Point{X: foot.X/zoom - at.X, Y: foot.Y/zoom - at.Y}
	return b, cam
}

// frontScene draws the tower at night as the map draws a scene, with
// something in front of it when f is given: the tower, its lights noted,
// then f in the depth sort, the wash, the lights, and f over the frame.
func (g *Game) frontScene(zoom float64, lit bool, f *inFront) *image.NRGBA {
	img := sheetImage()
	defer img.Deallocate()
	img.Fill(colorVoid)
	b, cam := sheetTower(zoom, lit)
	g.mapLabels = g.mapLabels[:0]
	g.beginLights(sheetCellSize, sheetCellSize)
	r := g.kitRising(img, cam, buildingPiece(b), g.footOf(cam, b), nil, 1)
	g.lightBuilding(cam, b)
	if f != nil && f.sorted != nil {
		f.sorted(g, img, cam, r, g.footOf(cam, b))
	}
	wash(img, 0, 0, sheetCellSize, sheetCellSize)
	g.endLights(img)
	if f != nil && f.over != nil {
		f.over(g, img, g.footOf(cam, b))
		g.drawMapLabels(img)
	}
	g.mapLabels = g.mapLabels[:0]
	return readSheet(img)
}

// frontAlone draws only what f puts in front of the tower, where it
// would stand, over nothing: its alpha is what it covers.
func (g *Game) frontAlone(zoom float64, f *inFront) *image.NRGBA {
	img := sheetImage()
	defer img.Deallocate()
	scratch := sheetImage()
	defer scratch.Deallocate()
	b, cam := sheetTower(zoom, false)
	r := g.kitRising(scratch, cam, buildingPiece(b), g.footOf(cam, b), nil, 1)
	g.mapLabels = g.mapLabels[:0]
	if f.sorted != nil {
		f.sorted(g, img, cam, r, g.footOf(cam, b))
	}
	if f.over != nil {
		f.over(g, img, g.footOf(cam, b))
		g.drawMapLabels(img)
	}
	g.mapLabels = g.mapLabels[:0]
	return readSheet(img)
}

// sheetImage is a check's picture, on a texture of its own: packed into
// the shared atlas, each would sit at a different offset, and a soft
// edge rounds differently there (item 191).
func sheetImage() *ebiten.Image {
	return ebiten.NewImageWithOptions(image.Rect(0, 0, sheetCellSize, sheetCellSize), &ebiten.NewImageOptions{Unmanaged: true})
}

func readSheet(img *ebiten.Image) *image.NRGBA {
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// unplacedInFront names each thing that stands in front of the tower
// only where its words are legible, and that its layout leaves off at
// this zoom.
func (g *Game) unplacedInFront(zoom float64) []string {
	var out []string
	for _, f := range inFronts(zoom) {
		if !g.placedInFront(zoom, &f) {
			out = append(out, f.name)
		}
	}
	return out
}

// placedInFront is whether a thing's layout puts it in front of the
// tower at this zoom.
func (g *Game) placedInFront(zoom float64, f *inFront) bool {
	if f.placed == nil {
		return true
	}
	b, cam := sheetTower(zoom, true)
	frame, ok := g.buildingFrame(cam, b, false, true, 0)
	if !ok {
		return false
	}
	return f.placed(g, cam, g.spriteRect(cam, &frame), g.footOf(cam, b))
}
