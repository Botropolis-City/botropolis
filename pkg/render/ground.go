package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// The map view (below city.DetailZoom) is flat colour, the way Factorio's
// chart draws entities as blocks of their map colour; sprites only take over
// when a tile is at least twelve pixels wide.
const (
	maxGroundTiles = 30_000
)

var (
	colorGround      = color.NRGBA{0x38, 0x54, 0x2c, 0xff}
	colorGroundNight = color.NRGBA{0x1e, 0x2b, 0x1c, 0xff}
	colorKerb        = color.NRGBA{0x0c, 0x0e, 0x12, 0xff}
	colorMapDistrict = color.NRGBA{0x2b, 0x2f, 0x38, 0xff}
	colorMapDistHi   = color.NRGBA{0x3a, 0x40, 0x4c, 0xff}
	colorMapLit      = color.NRGBA{0x55, 0x9a, 0xe0, 0xff}
	colorMapIdle     = color.NRGBA{0x3a, 0x44, 0x58, 0xff}
	colorMapNeedsYou = colorNeedsYou
	colorMapUnatt    = color.NRGBA{0x8a, 0x6c, 0xd8, 0xff}
	colorMapResting  = color.NRGBA{0x44, 0x42, 0x48, 0xff}
	colorMapVacant   = color.NRGBA{0x4e, 0x48, 0x3c, 0xff}
	colorVacant      = color.NRGBA{0x6b, 0x62, 0x50, 0xff}
	colorVacantDim   = color.NRGBA{0x3a, 0x36, 0x2e, 0xff}
	colorMinimapView = color.NRGBA{0xff, 0xff, 0xff, 0xc0}
	colorWater       = color.NRGBA{0x5a, 0xa8, 0xd0, 0xff}
	colorWaterLight  = color.NRGBA{0xc8, 0xe8, 0xf8, 0xff}
	colorWaterNight  = color.NRGBA{0x28, 0x48, 0x68, 0xff}
	colorVoid        = color.NRGBA{0x12, 0x14, 0x1a, 0xff}
	colorPlazaFloor  = color.NRGBA{0x8a, 0x86, 0x7c, 0xff}
	colorMapPark     = color.NRGBA{0x2f, 0x4a, 0x2a, 0xff}
	colorMapPlaza    = color.NRGBA{0x5a, 0x57, 0x50, 0xff}
	colorLampGlow    = color.NRGBA{0xff, 0xd0, 0x70, 0x50}
	colorStreet      = color.NRGBA{0x4a, 0x4c, 0x52, 0xff}
	colorStreetLine  = color.NRGBA{0xd8, 0xd8, 0xc8, 0x80}
	colorHighlight   = color.NRGBA{0xff, 0xff, 0xff, 0xa0}

	townGrass = [3][2]int{{0, 0}, {1, 0}, {2, 0}}
	townTrees = [3][2]int{{4, 0}, {4, 1}, {5, 0}}
)

func groundScale(night bool) *ebiten.ColorScale {
	scale := &ebiten.ColorScale{}
	if night {
		scale.SetR(0.28)
		scale.SetG(0.32)
		scale.SetB(0.3)
	} else {
		scale.SetR(0.44)
		scale.SetG(0.47)
		scale.SetB(0.42)
	}
	return scale
}

// ground fills the viewport: nothing beyond the map's edge, flat colour
// in the map view, grass tiles with the plan's trees when the sprites
// are legible.
func (g *Game) ground(screen *ebiten.Image, cam *city.Camera, c *city.City, width, height float64, detailed bool) {
	screen.Fill(colorVoid)
	bounds := c.Bounds()
	if bounds.Area() == 0 {
		return
	}
	g.shore(screen, cam, c)
	flat := colorGround
	if c.Night {
		flat = colorGroundNight
	}
	g.rect(screen, cam, bounds, g.flat(flat))
	if g.sprites == nil || !detailed {
		return
	}
	min := cam.ScreenToWorld(city.Point{})
	max := cam.ScreenToWorld(city.Point{X: width, Y: height})
	col0, col1 := int(math.Floor(math.Max(min.X, bounds.Min.X)/city.Tile)), int(math.Ceil(math.Min(max.X, bounds.Max.X)/city.Tile))
	row0, row1 := int(math.Floor(math.Max(min.Y, bounds.Min.Y)/city.Tile)), int(math.Ceil(math.Min(max.Y, bounds.Max.Y)/city.Tile))
	if (col1-col0)*(row1-row0) > maxGroundTiles {
		return
	}
	scale := groundScale(c.Night)
	for row := row0; row < row1; row++ {
		for col := col0; col < col1; col++ {
			r := city.RectAt(float64(col)*city.Tile, float64(row)*city.Tile, city.Tile, city.Tile)
			g.drawTile(screen, cam, g.sprites.town.tile(townGrass[0][0], townGrass[0][1]), r, scale)
		}
	}
	for _, t := range c.Trees {
		if !g.scene.Scenery() {
			break
		}
		centre := t.At
		r := city.RectAt(centre.X-city.Tile/2, centre.Y-city.Tile/2, city.Tile, city.Tile)
		tree := townTrees[((t.Cell.Row%len(townTrees))+len(townTrees))%len(townTrees)]
		g.drawTile(screen, cam, g.sprites.town.tile(tree[0], tree[1]), r, scale)
	}
}

// flatCells draws the river, lake and streets as flat cells in the
// top-down view, the same grid the isometric view tiles.
func (g *Game) flatCells(screen *ebiten.Image, cam *city.Camera, c *city.City) {
	water := colorWater
	if c.Night {
		water = colorWaterNight
	}
	water = g.flat(water)
	for _, rc := range c.RiverCells {
		g.rect(screen, cam, rc.Cell.Rect(), water)
	}
	if c.Plaza.Area() > 0 {
		g.rect(screen, cam, c.Plaza, g.flat(colorPlazaFloor))
	}
	if c.Fountain.Area() > 0 {
		g.circle(screen, cam, c.Fountain.Center(), city.Tile, water)
	}
	street, streetLine := g.flat(colorStreet), g.flat(colorStreetLine)
	for _, sc := range c.StreetCells {
		r := sc.Cell.Rect()
		g.rect(screen, cam, r, street)
		centre := r.Center()
		half := city.BuildingSize / 2
		if sc.Mask&city.DirW != 0 {
			g.line(screen, cam, centre, city.Point{X: centre.X - half, Y: centre.Y}, 1, streetLine)
		}
		if sc.Mask&city.DirE != 0 {
			g.line(screen, cam, centre, city.Point{X: centre.X + half, Y: centre.Y}, 1, streetLine)
		}
		if sc.Mask&city.DirN != 0 {
			g.line(screen, cam, centre, city.Point{X: centre.X, Y: centre.Y - half}, 1, streetLine)
		}
		if sc.Mask&city.DirS != 0 {
			g.line(screen, cam, centre, city.Point{X: centre.X, Y: centre.Y + half}, 1, streetLine)
		}
	}
}

func (g *Game) stroke(screen *ebiten.Image, cam *city.Camera, r city.Rect, width float32, c color.NRGBA) {
	min := cam.WorldToScreen(r.Min)
	max := cam.WorldToScreen(r.Max)
	vector.StrokeRect(screen, float32(min.X), float32(min.Y), float32(max.X-min.X), float32(max.Y-min.Y), width, c, false)
}

// floorLabel writes fixed-size text on a plate so it reads on any ground;
// at is where the text starts, the plate wraps it.
func (g *Game) floorLabel(screen *ebiten.Image, at city.Point, s string, c color.NRGBA) {
	pad := g.theme.Grid() / 2
	g.plate(screen, ui.LayoutPlate(g.theme, s, city.Point{X: at.X - pad, Y: at.Y - g.theme.Grid()/4}, ui.Small, g.faces.Measure), s, c)
}

// blockColor is a building's map colour: the state alone, saturated enough
// to tell apart at a glance.
func blockColor(b *city.Building, seconds float64) color.NRGBA {
	switch {
	case b.Blinds:
		return colorMapResting
	case b.Vacant:
		return colorMapVacant
	case b.Pulse:
		return pulse(colorMapNeedsYou, seconds)
	case b.Session.State == state.Waiting:
		return ui.DefaultPalette.Waiting
	case b.Session.State == state.Unattended:
		return colorMapUnatt
	case b.Lit:
		return colorMapLit
	}
	return colorMapIdle
}

// block is the map-view building: a flat colour with a dark edge for the
// silhouette, its context as a strip up the right side, cranes and flags as
// dots.
func (g *Game) block(screen *ebiten.Image, cam *city.Camera, b *city.Building, seconds float64) {
	g.rect(screen, cam, b.Rect, g.flatBody(b, blockColor(b, seconds)))
	if b.Fill > 0 && !b.Blinds {
		gauge := city.RectAt(b.Rect.Max.X-b.Rect.Width()*0.22, b.Rect.Min.Y, b.Rect.Width()*0.22, b.Rect.Height())
		g.rect(screen, cam, gauge, colorGaugeBack)
		height := gauge.Height() * b.Fill
		g.rect(screen, cam, city.Rect{Min: city.Point{X: gauge.Min.X, Y: gauge.Max.Y - height}, Max: gauge.Max}, gaugeColor(b.Fill))
	}
	dot := b.Rect.Width() * 0.18
	for i := 0; g.shows(city.NetworkCranes) && i < min(b.Cranes, 3); i++ {
		g.rect(screen, cam, city.RectAt(b.Rect.Min.X+dot*0.5+float64(i)*dot*1.4, b.Rect.Min.Y+dot*0.5, dot, dot), colorCrane)
	}
	if b.Flags > 0 {
		g.rect(screen, cam, city.RectAt(b.Rect.Max.X-b.Rect.Width()*0.22-dot*1.5, b.Rect.Min.Y+dot*0.5, dot, dot), colorFlag)
	}
	g.stroke(screen, cam, b.Rect, 1, colorKerb)
}

// shack is the parked session's 2x2 lot when sprites are on.
func (g *Game) shack(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	scale := &ebiten.ColorScale{}
	scale.SetR(0.5)
	scale.SetG(0.5)
	scale.SetB(0.55)
	for col := 0; col < 2; col++ {
		roof := townRoofGrey[col*2]
		wall := townWallStone[col*2]
		g.drawTile(screen, cam, g.sprites.town.tile(roof[0], roof[1]), cell(b.Rect, 2, 2, col, 0), scale)
		g.drawTile(screen, cam, g.sprites.town.tile(wall[0], wall[1]), cell(b.Rect, 2, 2, col, 1), scale)
	}
}

// minimap draws the whole city in a corner with the viewport marked, only
// when the viewport does not already show all of it.
func (g *Game) minimap(screen *ebiten.Image, box city.Rect) {
	th := g.theme
	m := g.scene.Minimap(box.Inset(th.Grid()))
	if m.Scale == 0 {
		return
	}
	c := g.scene.City()
	all := m.ProjectRect(c.Bounds())
	if m.View.Min.X <= all.Min.X && m.View.Min.Y <= all.Min.Y && m.View.Max.X >= all.Max.X && m.View.Max.Y >= all.Max.Y {
		return
	}
	g.roundPanel(screen, box)
	seconds := 0.0
	fill := func(r city.Rect, col color.NRGBA) {
		vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(math.Max(1, r.Width())), float32(math.Max(1, r.Height())), col, false)
	}
	fill(all, colorGroundNight)
	for _, park := range c.Parks {
		fill(m.ProjectRect(park.Rect), colorMapPark)
	}
	if c.Plaza.Area() > 0 {
		fill(m.ProjectRect(c.Plaza), colorMapPlaza)
	}
	for _, d := range c.Districts {
		r := m.ProjectRect(d.Rect)
		vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(math.Max(1, r.Width())), float32(math.Max(1, r.Height())), colorMapDistHi, false)
		for _, b := range d.Buildings {
			if b.Blinds {
				continue
			}
			p := m.Project(b.Rect.Center())
			vector.FillRect(screen, float32(p.X-1), float32(p.Y-1), 2, 2, blockColor(b, seconds), false)
		}
	}
	view := city.Rect{
		Min: city.Point{X: math.Max(box.Min.X, m.View.Min.X), Y: math.Max(box.Min.Y, m.View.Min.Y)},
		Max: city.Point{X: math.Min(box.Max.X, m.View.Max.X), Y: math.Min(box.Max.Y, m.View.Max.Y)},
	}
	vector.StrokeRect(screen, float32(view.Min.X), float32(view.Min.Y), float32(view.Width()), float32(view.Height()), 1, colorMinimapView, false)
}
