package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Each group is a line in neutral ink across its districts, labelled
// with its short name; its stations carry the tones of whose move each
// site is, so identity comes from the label and the route and status
// from colour (item 87).
var (
	colorLineInk    = color.NRGBA{0xe8, 0xe6, 0xdf, 0xff}
	colorLineCasing = color.NRGBA{0x1d, 0x20, 0x28, 0xff}
	// colorTeammate is a pull request opened outside the group's
	// sessions (item 90): grey, because whose move it is there is known
	// only as far as the group's own sessions saw it.
	colorTeammate = color.NRGBA{0x8a, 0x8d, 0x96, 0xff}
)

// lineFaded is how much of a line is left while another group has the
// focus.
const lineFaded = 0x40

// officeBox is a line's site office on screen: a square a little larger
// than a station of the given radius, so the head of the line reads as
// the place the work is run from (item 90).
func officeBox(at city.Point, radius float64) city.Rect {
	half := radius * 1.4
	return city.RectAt(at.X-half, at.Y-half, half*2, half*2)
}

// stationTone is a station's centre: its move's tone, or the line's own
// dark ink while it waits on another (item 90).
func stationTone(th ui.Theme, st city.Station) color.NRGBA {
	if st.Unlit {
		return colorLineCasing
	}
	if st.Teammate != nil {
		return colorTeammate
	}
	return th.Color(ui.MoveTone(st.Move))
}

// groupLines draws the groups' lines and their stations. With lit set
// it draws only the stations a hovered car keeps lit, to go over the
// faded rest, which a call without it draws (item 108).
func (g *Game) groupLines(screen *ebiten.Image, cam *city.Camera, lit bool) {
	lines := g.scene.Lines()
	focus := g.scene.FocusedGroup()
	width := math.Max(2, 3*cam.Zoom)
	radius := math.Max(4, 6*cam.Zoom)
	for li := range lines {
		l := &lines[li]
		ink, casing := colorLineInk, colorLineCasing
		if focus != "" && l.Group.ID != focus {
			ink.A, casing.A = lineFaded, lineFaded
		}
		points := make([]city.Point, len(l.Stations))
		for i, st := range l.Stations {
			points[i] = cam.WorldToScreen(st.At)
		}
		if lit {
			for i, st := range l.Stations {
				if g.stationLit(st.District) {
					g.station(screen, l, i, points[i], radius, ink, casing, stationTone(g.theme, st))
				}
			}
			continue
		}
		var office city.Point
		if l.Office != nil && len(points) > 0 {
			office = cam.WorldToScreen(l.Office.At)
			g.strokeLine(screen, float32(office.X), float32(office.Y), float32(points[0].X), float32(points[0].Y), float32(width+2), casing)
			g.strokeLine(screen, float32(office.X), float32(office.Y), float32(points[0].X), float32(points[0].Y), float32(width), ink)
		}
		for i := 1; i < len(points); i++ {
			a, b := points[i-1], points[i]
			if l.Stations[i].Dashed {
				g.dashed(screen, a, b, width+2, 6*cam.Zoom, casing)
				g.dashed(screen, a, b, width, 6*cam.Zoom, ink)
				continue
			}
			g.strokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(width+2), casing)
			g.strokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(width), ink)
		}
		for i, st := range l.Stations {
			if g.stationLit(st.District) {
				continue
			}
			tone := stationTone(g.theme, st)
			if focus != "" && l.Group.ID != focus {
				tone.A = lineFaded
			}
			g.station(screen, l, i, points[i], radius, ink, casing, tone)
		}
		if l.Office != nil && len(points) > 0 {
			box := officeBox(office, radius)
			move, _ := state.GroupMove([]state.Session{l.Office.Building.Session})
			tone := g.theme.Color(ui.MoveTone(move))
			if focus != "" && l.Group.ID != focus {
				tone.A = lineFaded
			}
			g.fillRect(screen, float32(box.Min.X-1.5), float32(box.Min.Y-1.5), float32(box.Width()+3), float32(box.Height()+3), casing)
			g.fillRect(screen, float32(box.Min.X), float32(box.Min.Y), float32(box.Width()), float32(box.Height()), ink)
			inner := box.Inset(box.Width() * 0.22)
			g.fillRect(screen, float32(inner.Min.X), float32(inner.Min.Y), float32(inner.Width()), float32(inner.Height()), tone)
		}
		if len(points) > 0 && g.labelsVisible() && l.Group.Short != "" {
			g.queueStationLabel(l.Stations[0].District, points[0], radius, l.Group.Short, ink, casing)
		}
	}
}

// station draws one station of a line and notes it under the pointer.
func (g *Game) station(screen *ebiten.Image, l *city.Line, i int, p city.Point, radius float64, ink, casing, tone color.NRGBA) {
	g.fillCircle(screen, float32(p.X), float32(p.Y), float32(radius+1.5), casing)
	g.fillCircle(screen, float32(p.X), float32(p.Y), float32(radius), ink)
	g.fillCircle(screen, float32(p.X), float32(p.Y), float32(radius*0.6), tone)
	g.noteHit(city.RectAt(p.X-radius, p.Y-radius, radius*2, radius*2),
		city.Hit{Station: &city.StationHit{Line: l, Index: i}})
}
