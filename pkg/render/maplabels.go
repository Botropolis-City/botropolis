package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// pendingLabel is a label asking for a place in this frame, with how to
// draw it wherever it is put. A sign has no draw: it stands in the world
// and was drawn there, and only claims its room.
type pendingLabel struct {
	mapLabel
	draw func(screen *ebiten.Image, at city.Rect)
	ink  color.NRGBA
}

func (g *Game) queueLabel(l pendingLabel) {
	g.mapLabels = append(g.mapLabels, l)
}

// queueBadge asks for a building's group badge under its foot.
func (g *Game) queueBadge(b *city.Building, foot city.Point) {
	bd, ok := badgeLayout(b, foot, g.measure)
	if !ok {
		return
	}
	g.queueLabel(pendingLabel{
		mapLabel: mapLabel{Kind: labelBadge, Rect: bd.Panel, Anchor: foot},
		draw: func(screen *ebiten.Image, at city.Rect) {
			g.drawBadge(screen, bd.moved(at.Min))
		},
		ink: colorBadgeInk,
	})
}

// queueStationLabel asks for a line's name beside its first station.
func (g *Game) queueStationLabel(station city.Point, radius float64, name string, ink, casing color.NRGBA) {
	at := city.Point{X: station.X + radius + 4, Y: station.Y - radius*2 - g.lineHeight()}
	w, h := g.measure(name)
	plate := city.RectAt(at.X-3, at.Y-1, w+6, h+2)
	g.queueLabel(pendingLabel{
		mapLabel: mapLabel{Kind: labelStation, Rect: plate, Anchor: station},
		draw: func(screen *ebiten.Image, p city.Rect) {
			vector.FillRect(screen, float32(p.Min.X), float32(p.Min.Y), float32(p.Width()), float32(p.Height()), casing, false)
			g.label(screen, city.Point{X: p.Min.X + 3, Y: p.Min.Y + 1}, name, ink)
		},
		ink: casing,
	})
}

// queueSign claims a district sign's room, board and post, so that no
// label is put over it.
func (g *Game) queueSign(board, post city.Rect) {
	for _, r := range []city.Rect{board, post} {
		g.queueLabel(pendingLabel{mapLabel: mapLabel{Kind: labelSign, Rect: r, Anchor: r.Center()}})
	}
}

// queueDistrictPlate asks for the plate that names a district whose
// sign is too small to read, or is under the pointer. It has a sign's
// right of way.
func (g *Game) queueDistrictPlate(d *city.District) {
	at := g.districtLabelAt(d)
	pad := g.theme.Grid() / 2
	p := ui.LayoutPlate(g.theme, d.Name, city.Point{X: at.X - pad, Y: at.Y - g.theme.Grid()/4}, ui.Small, g.faces.Measure)
	g.queueLabel(pendingLabel{
		mapLabel: mapLabel{Kind: labelPlate, Rect: p.Rect, Anchor: p.Rect.Center()},
		draw: func(screen *ebiten.Image, _ city.Rect) {
			g.plate(screen, p, d.Name, colorText)
		},
	})
}

// drawMapLabels lays out every label the frame asked for and draws them
// over the world, each with a leader back to what it names when it had
// to stand further off (item 102).
func (g *Game) drawMapLabels(screen *ebiten.Image) {
	asks := make([]mapLabel, len(g.mapLabels))
	for i, l := range g.mapLabels {
		asks[i] = l.mapLabel
	}
	laid := layoutLabels(asks, g.theme.Px(2))
	shown := make([]ShownLabel, 0, len(laid))
	for i, at := range laid {
		l := g.mapLabels[i]
		if !at.Shown {
			continue
		}
		shown = append(shown, ShownLabel{Kind: l.Kind.String(), Rect: at.At})
		if at.Leader {
			end := nearestOn(at.At, l.Anchor)
			vector.StrokeLine(screen, float32(l.Anchor.X), float32(l.Anchor.Y), float32(end.X), float32(end.Y), float32(g.theme.Px(1)), l.ink, true)
		}
		if l.draw != nil {
			l.draw(screen, at.At)
		}
	}
	g.mu.Lock()
	g.shownLabels = shown
	g.mu.Unlock()
	g.mapLabels = g.mapLabels[:0]
}

// toggleLabels turns every label off or on by flipping the settings
// row and applying it, so the key and the row save the same way and
// cannot disagree (item 102).
func (g *Game) toggleLabels() {
	value := config.LabelsHidden
	if !g.scene.Labels() {
		value = config.LabelsShown
	}
	row := ui.Setting{Key: config.KeyLabels, Label: "labels", Options: []string{config.LabelsShown, config.LabelsHidden}, Value: value}
	for i, s := range g.settings.Items {
		if s.Key == config.KeyLabels {
			g.settings.Items[i].Value = value
			row = g.settings.Items[i]
		}
	}
	g.applySetting(row)
}

// nearestOn is the point of r closest to p.
func nearestOn(r city.Rect, p city.Point) city.Point {
	return city.Point{X: min(max(p.X, r.Min.X), r.Max.X), Y: min(max(p.Y, r.Min.Y), r.Max.Y)}
}
