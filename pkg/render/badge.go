package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// A badge is a member's group on the map: a neutral plate under the
// building with the group's short name, and a dot in the tone of whose
// move it is (item 87).
const (
	badgeDrop = 10.0
	badgePad  = 3.0
)

var colorBadgeInk = color.NRGBA{0x1d, 0x20, 0x28, 0xff}

type badge struct {
	Text  string
	Tone  ui.Tone
	Panel city.Rect
	Dot   city.Point
	Ink   city.Point
}

func badgeLayout(b *city.Building, foot city.Point, measure func(string) (float64, float64)) (badge, bool) {
	if b.Group == nil || b.Group.Short == "" {
		return badge{}, false
	}
	w, h := measure(b.Group.Short)
	dot := h * 0.5
	width := badgePad + dot + badgePad + w + badgePad
	panel := city.RectAt(foot.X-width/2, foot.Y+badgeDrop, width, h+badgePad)
	return badge{
		Text:  b.Group.Short,
		Tone:  ui.MoveTone(b.Group.Move),
		Panel: panel,
		Dot:   city.Point{X: panel.Min.X + badgePad + dot/2, Y: panel.Min.Y + panel.Height()/2},
		Ink:   city.Point{X: panel.Min.X + badgePad + dot + badgePad, Y: panel.Min.Y + badgePad/2},
	}, true
}

// moved is the badge with its panel's corner at min, where the label
// layout put it (item 102).
func (bd badge) moved(min city.Point) badge {
	d := min.Sub(bd.Panel.Min)
	bd.Panel = city.Rect{Min: bd.Panel.Min.Add(d), Max: bd.Panel.Max.Add(d)}
	bd.Dot, bd.Ink = bd.Dot.Add(d), bd.Ink.Add(d)
	return bd
}

func (g *Game) drawBadge(screen *ebiten.Image, bd badge) {
	p := bd.Panel
	vector.FillRect(screen, float32(p.Min.X), float32(p.Min.Y), float32(p.Width()), float32(p.Height()), colorText, false)
	vector.StrokeRect(screen, float32(p.Min.X), float32(p.Min.Y), float32(p.Width()), float32(p.Height()), 1, colorBadgeInk, false)
	_, h := g.measure(bd.Text)
	g.fillCircle(screen, float32(bd.Dot.X), float32(bd.Dot.Y), float32(h*0.3), g.theme.Color(bd.Tone))
	g.label(screen, bd.Ink, bd.Text, colorBadgeInk)
}
