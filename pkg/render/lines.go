package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
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

func (g *Game) groupLines(screen *ebiten.Image, cam *city.Camera) {
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
		for i := 1; i < len(points); i++ {
			a, b := points[i-1], points[i]
			if l.Stations[i].Dashed {
				dashed(screen, a, b, width+2, 6*cam.Zoom, casing)
				dashed(screen, a, b, width, 6*cam.Zoom, ink)
				continue
			}
			vector.StrokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(width+2), casing, true)
			vector.StrokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(width), ink, true)
		}
		for i, st := range l.Stations {
			p := points[i]
			tone := stationTone(g.theme, st)
			if focus != "" && l.Group.ID != focus {
				tone.A = lineFaded
			}
			vector.FillCircle(screen, float32(p.X), float32(p.Y), float32(radius+1.5), casing, true)
			vector.FillCircle(screen, float32(p.X), float32(p.Y), float32(radius), ink, true)
			vector.FillCircle(screen, float32(p.X), float32(p.Y), float32(radius*0.6), tone, true)
			g.noteHit(city.RectAt(p.X-radius, p.Y-radius, radius*2, radius*2),
				city.Hit{Station: &city.StationHit{Line: l, Index: i}})
		}
		if len(points) > 0 && g.labelsVisible() && l.Group.Short != "" {
			at := city.Point{X: points[0].X + radius + 4, Y: points[0].Y - radius*2 - g.lineHeight()}
			w, h := g.measure(l.Group.Short)
			plate := city.RectAt(at.X-3, at.Y-1, w+6, h+2)
			vector.FillRect(screen, float32(plate.Min.X), float32(plate.Min.Y), float32(plate.Width()), float32(plate.Height()), casing, false)
			g.label(screen, at, l.Group.Short, ink)
		}
	}
}
