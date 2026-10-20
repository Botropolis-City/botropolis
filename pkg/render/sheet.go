package render

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

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
		vector.FillRect(clip, float32(cell.Min.X), float32(cell.Min.Y), float32(cell.Dx()), float32(cell.Dy()), colorNightOverlay, false)
		g.nightLights(clip, cam, &city.City{Districts: []*city.District{{Type: b.Type, Buildings: []*city.Building{b}}}})
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
		vector.FillRect(sheet, float32(margin), float32(y+header-4), float32(width-2*margin), 1, colorSheetRule, false)
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
	g    *Game
	zoom float64
	path string
	err  error
}

func (s *sheetRun) Update() error {
	img := s.g.drawSheet(s.zoom)
	b := img.Bounds()
	out := image.NewNRGBA(b)
	img.ReadPixels(out.Pix)
	f, err := os.Create(s.path)
	if err != nil {
		s.err = err
		return ebiten.Termination
	}
	s.err = errors.Join(png.Encode(f, out), f.Close())
	return ebiten.Termination
}

func (s *sheetRun) Draw(*ebiten.Image) {}

func (s *sheetRun) Layout(int, int) (int, int) { return 64, 64 }

// Sheet writes the building sheet at a zoom to a PNG. It opens a
// window for the one frame it needs, so run it on a virtual display.
func Sheet(path string, zoom float64) error {
	k, err := loadKits()
	if err != nil {
		return err
	}
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	if err != nil {
		return err
	}
	run := &sheetRun{g: &Game{kits: k, faces: f, theme: th, scene: city.NewScene(nil)}, zoom: zoom, path: path}
	ebiten.SetWindowSize(64, 64)
	if err := ebiten.RunGame(run); err != nil && !errors.Is(err, ebiten.Termination) {
		return err
	}
	return run.err
}
