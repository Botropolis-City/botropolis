package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

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
	// faded is a label outside what the highlight ties together, drawn
	// receded as the world around it is (item 102).
	faded bool
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
		ink:   colorBadgeInk,
		faded: g.lit.FadesDistrict(g.scene.City().DistrictOf(b)),
	})
}

// queueStationLabel asks for a line's name beside its first station.
func (g *Game) queueStationLabel(d *city.District, station city.Point, radius float64, name string, ink, casing color.NRGBA) {
	at := city.Point{X: station.X + radius + 4, Y: station.Y - radius*2 - g.lineHeight()}
	w, h := g.measure(name)
	plate := city.RectAt(at.X-3, at.Y-1, w+6, h+2)
	g.queueLabel(pendingLabel{
		mapLabel: mapLabel{Kind: labelStation, Rect: plate, Anchor: station},
		draw: func(screen *ebiten.Image, p city.Rect) {
			g.paint.Box(screen, float32(p.Min.X), float32(p.Min.Y), float32(p.Width()), float32(p.Height()), casing)
			g.label(screen, city.Point{X: p.Min.X + 3, Y: p.Min.Y + 1}, name, ink)
		},
		ink:   casing,
		faded: g.lit.FadesDistrict(d),
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
	p := g.floorPlate(g.districtLabelAt(d), d.Name)
	g.queueLabel(pendingLabel{
		mapLabel: mapLabel{Kind: labelPlate, Rect: p.Rect, Anchor: p.Rect.Center()},
		draw: func(screen *ebiten.Image, at city.Rect) {
			g.plate(screen, p.Moved(at.Min), d.Name, colorText)
		},
		faded: g.lit.FadesDistrict(d),
	})
}

// queueFloorLabel asks for a plate that floorLabel would have drawn at
// at, anchored there.
func (g *Game) queueFloorLabel(kind labelKind, at city.Point, s string, ink color.NRGBA, faded bool) {
	p := g.floorPlate(at, s)
	g.queueLabel(pendingLabel{
		mapLabel: mapLabel{Kind: kind, Rect: p.Rect, Anchor: at},
		draw: func(screen *ebiten.Image, r city.Rect) {
			g.plate(screen, p.Moved(r.Min), s, ink)
		},
		ink:   g.theme.Palette.Panel,
		faded: faded,
	})
}

// queueRoadTags asks for each road's traffic tag, where the static
// layer used to paint it under everything (item 102).
func (g *Game) queueRoadTags(c *city.City, cam *city.Camera) {
	if !g.labelsVisible() {
		return
	}
	if g.kits == nil {
		for i := range c.Roads {
			road := &c.Roads[i]
			mid := city.Point{X: (road.A.X + road.B.X) / 2, Y: (road.A.Y + road.B.Y) / 2}
			g.queueRoadTag(road, cam.WorldToScreen(mid).Add(city.Point{X: g.theme.Px(4), Y: -g.lineHeight() + g.theme.Px(2)}))
		}
		return
	}
	for i := range c.Streets {
		street := &c.Streets[i]
		mid := street.Path[len(street.Path)/2]
		g.queueRoadTag(street.Road, cam.WorldToScreen(mid).Add(city.Point{X: g.theme.Px(6), Y: -g.lineHeight() - g.theme.Px(6)}))
	}
}

// queueRoadTag asks for one road's tag at at, faded unless the road is
// in the highlight.
func (g *Game) queueRoadTag(road *city.RoadLine, at city.Point) {
	g.queueFloorLabel(labelTag, at, road.Label(), colorDim, g.lit.FadesRoad(road))
}

// drawMapLabels lays out every label the frame asked for and draws them
// over the world, each with a leader back to what it names when it had
// to stand further off (item 102).
func (g *Game) drawMapLabels(screen *ebiten.Image) {
	if g.labelFrame.Area() > 0 {
		g.mapLabels = reachable(g.mapLabels, g.labelFrame, g.theme.Px(2))
	}
	asks := make([]mapLabel, len(g.mapLabels))
	for i, l := range g.mapLabels {
		asks[i] = l.mapLabel
	}
	g.laidOut = len(asks)
	laid := layoutLabels(asks, g.theme.Px(2))
	shown := make([]ShownLabel, 0, len(laid))
	// The faded labels' layer is made and cleared only when one is
	// faded: clearing it every frame was a draw and a target switch for
	// nothing (item 193).
	var faded *ebiten.Image
	for i, at := range laid {
		l := g.mapLabels[i]
		if !at.Shown {
			continue
		}
		shown = append(shown, ShownLabel{Kind: l.Kind.String(), Rect: at.At, Faded: l.faded})
		dst := screen
		if l.faded {
			if faded == nil {
				faded = g.fadeLayer(screen)
			}
			dst = faded
		}
		if at.Leader {
			end := nearestOn(at.At, l.Anchor)
			g.strokeLine(dst, float32(l.Anchor.X), float32(l.Anchor.Y), float32(end.X), float32(end.Y), float32(g.theme.Px(1)), l.ink)
		}
		if l.draw != nil {
			if l.Kind == labelTag {
				g.drawing.tag()
			}
			g.drawLabel(dst, l.draw, at.At)
		}
	}
	if faded != nil {
		var op ebiten.DrawImageOptions
		op.ColorScale.ScaleAlpha(labelFadedAlpha)
		screen.DrawImage(faded, &op)
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

// labelFadedAlpha is how much of a label outside the highlight is left.
const labelFadedAlpha = 0.3

// fadeLayer is a clear layer the size of the screen that the faded
// labels are drawn into, so each is faded as a whole rather than piece
// by piece over itself.
func (g *Game) fadeLayer(screen *ebiten.Image) *ebiten.Image {
	b := screen.Bounds()
	if g.labelFade == nil || g.labelFade.Bounds() != b {
		if g.labelFade != nil {
			g.labelFade.Deallocate()
		}
		g.labelFade = sizedImage(nil, b.Size())
	}
	g.labelFade.Clear()
	return g.labelFade
}

// nearestOn is the point of r closest to p.
func nearestOn(r city.Rect, p city.Point) city.Point {
	return city.Point{X: min(max(p.X, r.Min.X), r.Max.X), Y: min(max(p.Y, r.Min.Y), r.Max.Y)}
}
