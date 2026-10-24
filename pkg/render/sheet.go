package render

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// A building sheet is every piece a session's building can be, drawn
// each way its windows can look: lived in or boarded up, by day and by
// night, with the calls isoBuilding and the night pass make. Item 166.

type sheetLook struct {
	name    string
	boarded bool
	night   bool
}

var sheetLooks = []sheetLook{
	{"normal, day", false, false},
	{"normal, night", false, true},
	{"boarded up, day", true, false},
	{"boarded up, night", true, true},
}

var sheetKinds = []struct {
	name string
	kind city.DistrictType
}{
	{"residential", city.ResidentialDistrict},
	{"shopping", city.ShoppingDistrict},
	{"office", city.OfficeDistrict},
	{"industrial", city.IndustrialDistrict},
}

var (
	colorSheetInk   = color.NRGBA{0xe8, 0xe6, 0xe0, 0xff}
	colorSheetFaint = color.NRGBA{0x9a, 0x9c, 0xa6, 0xff}
	colorSheetRule  = color.NRGBA{0x2a, 0x2e, 0x38, 0xff}
)

// sheetBuilding is the building one cell of the sheet draws: a kind at a
// tier, or its ruins at tier 0, lived in or boarded up. Ruins are only
// ever a resting session's (the ledger's), so they are never lit: their
// "normal" is resting behind blinds.
func sheetBuilding(kind city.DistrictType, tier int, boarded bool) *city.Building {
	s := state.Session{ID: fmt.Sprintf("sheet-%s-%d-%v", kind, tier, boarded), State: state.Working, Storeys: max(tier, 1)}
	rung := state.RungLive
	switch {
	case boarded:
		s.State, rung = state.Resting, state.RungShuttered
	case tier == 0:
		s.State, rung = state.Resting, state.RungResting
	}
	s.ConversationGone = tier == 0
	return &city.Building{
		Type:      kind,
		Rect:      city.RectAt(0, 0, city.BuildingSize, city.BuildingSize),
		Session:   s,
		Lit:       state.Live(s.State),
		Rung:      rung,
		Blinds:    rung != state.RungLive,
		BoardedUp: rung == state.RungShuttered,
	}
}

// sheetCell draws one building with its foot on a screen point, as the
// map draws it, and lights it as the night pass does when it is night.
func (g *Game) sheetCell(screen *ebiten.Image, zoom float64, b *city.Building, foot city.Point, night bool, cell image.Rectangle) {
	cam := &city.Camera{Zoom: zoom}
	at := cam.Project(b.Rect.Center())
	cam.Offset = city.Point{X: foot.X/zoom - at.X, Y: foot.Y/zoom - at.Y}
	clip := screen.SubImage(cell).(*ebiten.Image)
	var tint *ebiten.ColorScale
	if b.Blinds {
		tint = blindsTint()
	}
	name := buildingPiece(b)
	g.kitRising(clip, cam, name, g.footOf(cam, b), tint, 1)
	if dress := dressLot(b, -1); dress.Dark {
		g.shutter(clip, cam, b, name, dress.Boards)
	}
	if night {
		wash(clip, float32(cell.Min.X), float32(cell.Min.Y), float32(cell.Dx()), float32(cell.Dy()))
		if lights, ok := g.windowLights(cam, b, 1, 1); ok {
			g.drawWindows(clip, cam, b, lights, 1)
		}
	}
}

// drawSheet lays the sheet out: a block per kind, a row per look and a
// column per tier with the ruins last, labelled down the side and along
// the top.
func (g *Game) drawSheet(zoom float64) *ebiten.Image {
	cam := &city.Camera{Zoom: zoom}
	var tallest, widest int
	for _, k := range sheetKinds {
		for tier := 0; tier <= state.MaxStoreys; tier++ {
			size := g.kitSize(cam, buildingPiece(sheetBuilding(k.kind, tier, false)))
			tallest, widest = max(tallest, size.Y), max(widest, size.X)
		}
	}
	const margin, header, side = 16, 34, 210
	cellW, cellH := widest+24, tallest+28
	cols := state.MaxStoreys + 1
	width := side + cols*cellW + margin
	height := header + len(sheetKinds)*(len(sheetLooks)*cellH+header) + margin
	sheet := ebiten.NewImage(width, height)
	sheet.Fill(colorVoid)
	for c := range cols {
		label := fmt.Sprintf("tier %d", c+1)
		if c == state.MaxStoreys {
			label = "ruins (never lit)"
		}
		g.text(sheet, city.Point{X: float64(side + c*cellW + 8), Y: 8}, label, ui.Body, colorSheetInk)
	}
	y := header
	for _, k := range sheetKinds {
		g.text(sheet, city.Point{X: margin, Y: float64(y + 6)}, k.name, ui.Title, colorSheetInk)
		g.paint.Box(sheet, float32(margin), float32(y+header-4), float32(width-2*margin), 1, colorSheetRule)
		y += header
		for _, look := range sheetLooks {
			g.text(sheet, city.Point{X: margin, Y: float64(y + cellH/2 - 8)}, look.name, ui.Body, colorSheetFaint)
			for c := range cols {
				tier := c + 1
				if c == state.MaxStoreys {
					tier = 0
				}
				cell := image.Rect(side+c*cellW, y, side+(c+1)*cellW, y+cellH)
				foot := city.Point{X: float64(cell.Min.X + cellW/2), Y: float64(cell.Max.Y - 14)}
				g.sheetCell(sheet, zoom, sheetBuilding(k.kind, tier, look.boarded), foot, look.night, cell.Inset(2))
			}
			y += cellH
		}
	}
	return sheet
}

type sheetRun struct {
	g   *Game
	job func(g *Game) error
	err error
}

func (s *sheetRun) Update() error {
	s.err = s.job(s.g)
	return ebiten.Termination
}

func (s *sheetRun) Draw(*ebiten.Image) {}

func (s *sheetRun) Layout(int, int) (int, int) { return 64, 64 }

// runSheet runs job once inside the loop, which is the only place a
// picture can be read back. It opens a window for the one frame it
// needs, so run it on a virtual display.
func runSheet(job func(g *Game) error) error {
	k, err := loadKits()
	if err != nil {
		return err
	}
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	if err != nil {
		return err
	}
	run := &sheetRun{g: &Game{kits: k, faces: f, theme: th, scene: city.NewScene(nil)}, job: job}
	ebiten.SetWindowSize(64, 64)
	if err := ebiten.RunGame(run); err != nil && !errors.Is(err, ebiten.Termination) {
		return err
	}
	return run.err
}

// Sheet writes the building sheet at a zoom to a PNG.
func Sheet(path string, zoom float64) error {
	return runSheet(func(g *Game) error {
		img := g.drawSheet(zoom)
		out := image.NewNRGBA(img.Bounds())
		img.ReadPixels(out.Pix)
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		return errors.Join(png.Encode(f, out), f.Close())
	})
}

// litMargin is how far outside a pane a lit pixel may fall, in screen
// pixels: the layer is drawn with linear filtering, which spreads an
// edge by one.
const litMargin = 1

// CheckLights draws every building the sheet does, at night, with its
// lights on and off, and reports each one whose lights changed a pixel
// further than litMargin from its panes, or that lit at all when it is
// boarded up or ruins. Item 166.
func CheckLights(zoom float64) ([]string, error) {
	var problems []string
	err := runSheet(func(g *Game) error {
		for _, k := range sheetKinds {
			for tier := 0; tier <= state.MaxStoreys; tier++ {
				for _, boarded := range []bool{false, true} {
					b := sheetBuilding(k.kind, tier, boarded)
					on, off := g.sheetNight(zoom, b, true), g.sheetNight(zoom, b, false)
					panes, foot := g.sheetPanes(zoom, b)
					stray, lit := 0, 0
					bounds := on.Bounds()
					for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
						for x := bounds.Min.X; x < bounds.Max.X; x++ {
							if on.NRGBAAt(x, y) == off.NRGBAAt(x, y) {
								continue
							}
							lit++
							if !nearPane(panes, x, y, litMargin) {
								stray++
							}
						}
					}
					name := fmt.Sprintf("%s (boarded %v) at %v", buildingPiece(b), boarded, foot)
					if stray > 0 {
						problems = append(problems, fmt.Sprintf("%s: %d lit pixels outside its panes", name, stray))
					}
					if lit > 0 && !b.Lit {
						problems = append(problems, fmt.Sprintf("%s: lit %d pixels though it is not lived in", name, lit))
					}
					if lit == 0 && b.Lit {
						problems = append(problems, fmt.Sprintf("%s: lived in but lit nothing", name))
					}
					if b.Lit && tier >= 2 {
						problems = append(problems, g.checkRising(zoom, b)...)
					}
				}
			}
		}
		problems = append(problems, g.checkOcclusion(zoom)...)
		problems = append(problems, g.checkInFront(zoom)...)
		problems = append(problems, g.checkLandmarks(zoom)...)
		return nil
	})
	return problems, err
}

// checkOcclusion draws a lit tower with a dark building standing in
// front of it, as the map draws a scene, lights on and off, and reports
// any lit pixel on the building in front: a window behind it must not
// show through it.
func (g *Game) checkOcclusion(zoom float64) []string {
	back := sheetBuilding(city.ResidentialDistrict, state.MaxStoreys, false)
	front := sheetBuilding(city.OfficeDistrict, state.MaxStoreys, false)
	front.Lit = false
	front.Session.ID = "sheet-front"
	backFoot := city.Point{X: sheetCellSize / 2, Y: sheetCellSize * 0.6}
	frontFoot := city.Point{X: backFoot.X + 10*zoom, Y: backFoot.Y + 40*zoom}
	scene := func(lit bool) *image.NRGBA {
		b := *back
		b.Lit = lit
		return g.sheetScene(zoom, []*city.Building{&b, front}, []city.Point{backFoot, frontFoot})
	}
	on, off := scene(true), scene(false)
	mask := g.sheetScene(zoom, []*city.Building{front}, []city.Point{frontFoot})
	blank := g.sheetScene(zoom, nil, nil)
	return seenThroughFront("a lit tower behind a dark building", on, off, mask, blank, g.solidMargin(zoom))
}

// solidMarginFor is how many pixels in from a building's soft edge the
// occlusion checks take it as solid, for a sprite drawn up drawnUp times
// its cut. A cut's rim is 2 px at every level; drawn up, filtering widens
// it, and at zoom 2 with z1.5 (1.33x) a pixel 3 in is only about 60%
// opaque, which read as a window showing through. The checks had only
// run at zoom 1 and 2, where nothing was drawn up, until item 162's
// slice 14 replaced z2 with z1.5; zoom 1.5, z1 drawn up, failed the same
// way before it.
func solidMarginFor(drawnUp float64) int {
	if drawnUp <= 1.001 {
		return 2
	}
	return int(math.Ceil(2*drawnUp)) + 1
}

// solidMargin is solidMarginFor the kit level the camera draws at zoom.
func (g *Game) solidMargin(zoom float64) int {
	if g.kits == nil {
		return solidMarginFor(1)
	}
	atlas := g.kits.pick(zoom)
	if atlas == nil {
		return solidMarginFor(1)
	}
	return solidMarginFor(zoom / atlas.Zoom)
}

// seenThroughFront reports any pixel the lights changed, between on and
// off, that lies solidly inside the dark building in front, which mask
// draws alone over blank: a window behind it must not show through it.
// margin is how far in from that building's soft edge it is solid
// (solidMargin): at the edge, filtered, it is part see-through, and was
// before.
func seenThroughFront(name string, on, off, mask, blank *image.NRGBA, margin int) []string {
	solid := func(x, y int) bool {
		for dy := -margin; dy <= margin; dy++ {
			for dx := -margin; dx <= margin; dx++ {
				if mask.NRGBAAt(x+dx, y+dy) == blank.NRGBAAt(x+dx, y+dy) {
					return false
				}
			}
		}
		return true
	}
	through, lit := 0, 0
	bounds := on.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if on.NRGBAAt(x, y) == off.NRGBAAt(x, y) {
				continue
			}
			lit++
			if solid(x, y) && brighter(on.NRGBAAt(x, y), off.NRGBAAt(x, y)) > seenThrough {
				through++
			}
		}
	}
	switch {
	case lit == 0:
		return []string{name + ": lit nothing"}
	case through > 0:
		return []string{fmt.Sprintf("%s: %d lit pixels show through the building in front", name, through)}
	}
	return nil
}

// seenThrough is how much brighter, in any channel, a pixel must be with
// the lights on for a window to show through: a lit window is about a
// hundred levels; filtering at an edge moves a pixel by one or two.
const seenThrough = 16

// brighter is how much brighter on is than off, in its brightest channel.
func brighter(on, off color.NRGBA) int {
	return max(int(on.R)-int(off.R), int(on.G)-int(off.G), int(on.B)-int(off.B))
}

// sheetScene draws buildings at night, back to front, the way the map
// draws a scene: each building as isoBuilding draws it, then the wash,
// then the lights.
func (g *Game) sheetScene(zoom float64, buildings []*city.Building, feet []city.Point) *image.NRGBA {
	img := ebiten.NewImage(sheetCellSize, sheetCellSize)
	defer img.Deallocate()
	img.Fill(colorVoid)
	g.beginLights(sheetCellSize, sheetCellSize)
	for i, b := range buildings {
		cam := &city.Camera{Zoom: zoom}
		at := cam.Project(b.Rect.Center())
		cam.Offset = city.Point{X: feet[i].X/zoom - at.X, Y: feet[i].Y/zoom - at.Y}
		g.kitRising(img, cam, buildingPiece(b), g.footOf(cam, b), nil, 1)
		g.lightBuilding(cam, b)
	}
	wash(img, 0, 0, sheetCellSize, sheetCellSize)
	g.endLights(img)
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// checkRising draws a building going up with only its ground floor
// finished, lights on and off, and reports lit pixels that are not on
// its ground floor's panes as the rising sprite shows them (item 155).
func (g *Game) checkRising(zoom float64, b *city.Building) []string {
	cam, foot := g.sheetCam(zoom, b)
	storeys, storey, plinth := g.storeyFrame(cam, b, g.kitSize(cam, buildingPiece(b)))
	rise := (plinth + 1.2*storey) / (plinth + float64(storeys)*storey)
	on, off := g.sheetRising(cam, b, rise, true), g.sheetRising(cam, b, rise, false)
	atlas := g.kits.pick(zoom)
	sprite, _ := atlas.Sprite(buildingPiece(b), cam.Heading)
	of := paneStoreys(sprite, atlas.Zoom, cam.Heading)
	var ground [][4]city.Point
	for i, q := range g.facadePanes(cam, buildingPiece(b), g.footOf(cam, b)) {
		if of[i] != 0 {
			continue
		}
		for j := range q {
			q[j].Y = foot.Y + (q[j].Y-foot.Y)*rise
		}
		ground = append(ground, q)
	}
	stray, lit := 0, 0
	bounds := on.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if on.NRGBAAt(x, y) == off.NRGBAAt(x, y) {
				continue
			}
			lit++
			if !nearPane(ground, x, y, litMargin) {
				stray++
			}
		}
	}
	name := fmt.Sprintf("%s going up, its ground floor built", buildingPiece(b))
	switch {
	case lit == 0:
		return []string{name + ": lit nothing"}
	case stray > 0:
		return []string{fmt.Sprintf("%s: %d lit pixels off its ground floor's panes", name, stray)}
	}
	return nil
}

// sheetCam is the camera sheetNight draws a building with, and its foot.
func (g *Game) sheetCam(zoom float64, b *city.Building) (*city.Camera, city.Point) {
	cam := &city.Camera{Zoom: zoom}
	foot := city.Point{X: sheetCellSize / 2, Y: sheetCellSize - 16}
	at := cam.Project(b.Rect.Center())
	cam.Offset = city.Point{X: foot.X/zoom - at.X, Y: foot.Y/zoom - at.Y}
	return cam, foot
}

// sheetRising is a building drawn at night risen part of the way, with
// its lights on or off.
func (g *Game) sheetRising(cam *city.Camera, b *city.Building, rise float64, lights bool) *image.NRGBA {
	img := ebiten.NewImage(sheetCellSize, sheetCellSize)
	defer img.Deallocate()
	img.Fill(colorVoid)
	g.kitRising(img, cam, buildingPiece(b), g.footOf(cam, b), nil, rise)
	wash(img, 0, 0, sheetCellSize, sheetCellSize)
	if l, ok := g.windowLights(cam, b, rise, 1); ok && lights {
		g.drawWindows(img, cam, b, l, rise)
	}
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// sheetCellSize is the picture each building is checked in.
const sheetCellSize = 512

// sheetNight is one building drawn at night on its own, with its lights
// on or off. It draws on a texture of its own: packed into the shared
// atlas, each frame would sit at a different offset, and a shape's
// soft edge rounds a level differently there (item 191), which the
// check would read as a light.
func (g *Game) sheetNight(zoom float64, b *city.Building, lights bool) *image.NRGBA {
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, sheetCellSize, sheetCellSize), &ebiten.NewImageOptions{Unmanaged: true})
	defer img.Deallocate()
	img.Fill(colorVoid)
	c := *b
	c.Lit = b.Lit && lights
	g.sheetCell(img, zoom, &c, city.Point{X: sheetCellSize / 2, Y: sheetCellSize - 16}, true, img.Bounds())
	out := image.NewNRGBA(img.Bounds())
	img.ReadPixels(out.Pix)
	return out
}

// sheetPanes is a building's panes where sheetNight draws it.
func (g *Game) sheetPanes(zoom float64, b *city.Building) ([][4]city.Point, city.Point) {
	cam, foot := g.sheetCam(zoom, b)
	return g.facadePanes(cam, buildingPiece(b), g.footOf(cam, b)), foot
}

// nearPane is whether a pixel's centre lies within margin of a pane.
func nearPane(panes [][4]city.Point, x, y int, margin float64) bool {
	for _, q := range panes {
		for _, d := range []city.Point{{}, {X: -margin}, {X: margin}, {Y: -margin}, {Y: margin}} {
			if inScreenQuad(q, city.Point{X: float64(x) + 0.5 + d.X, Y: float64(y) + 0.5 + d.Y}) {
				return true
			}
		}
	}
	return false
}

func inScreenQuad(q [4]city.Point, p city.Point) bool {
	sign := 0.0
	for i := range q {
		a, b := q[i], q[(i+1)%4]
		cross := (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
		if cross == 0 {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if (cross > 0) != (sign > 0) {
			return false
		}
	}
	return true
}
