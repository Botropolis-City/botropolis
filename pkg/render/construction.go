package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// The fence, scaffolding and crane of a build on a new session's lot.
// Item 89.

var (
	colorFence     = color.NRGBA{0xe8, 0xb0, 0x3a, 0xff}
	colorScaffold  = color.NRGBA{0xb8, 0xb4, 0xa8, 0xff}
	colorSiteCrane = color.NRGBA{0xf0, 0xc0, 0x30, 0xff}
)

// segment is one straight stroke on screen; pole marks an upright that
// stands on the ground.
type segment struct {
	a, b  city.Point
	width float64
	pole  bool
	// plank is a deck board on a scaffold, brace a diagonal tube across
	// one of its bays, and corner a pole at its corner (item 89).
	plank, brace, corner bool
}

// siteLines is the build's strokes on a lot whose screen corners are
// north, east, south and west, under a building whose finished roof is
// at screen height roofline.
func siteLines(lot [4]city.Point, roofline float64, site city.Site, zoom float64) (fence, scaffold, crane []segment) {
	centre := city.Point{X: (lot[1].X + lot[3].X) / 2, Y: (lot[0].Y + lot[2].Y) / 2}
	toward := func(p, q city.Point, f float64) city.Point {
		return city.Point{X: p.X + (q.X-p.X)*f, Y: p.Y + (q.Y-p.Y)*f}
	}
	up := func(p city.Point, h float64) city.Point { return city.Point{X: p.X, Y: p.Y - h} }

	if site.Fence > 0 {
		h := 4 * zoom * site.Fence
		for i := range lot {
			next := lot[(i+1)%len(lot)]
			fence = append(fence, segment{a: lot[i], b: up(lot[i], h), width: zoom, pole: true})
			fence = append(fence, segment{a: up(lot[i], h), b: up(next, h), width: zoom})
		}
	}
	if site.Scaffold > 0 {
		var tops []city.Point
		for _, corner := range lot {
			foot := toward(corner, centre, 0.15)
			top := up(foot, (foot.Y-roofline)*site.Scaffold)
			scaffold = append(scaffold, segment{a: foot, b: top, width: zoom, pole: true})
			tops = append(tops, top)
		}
		for i := range tops {
			scaffold = append(scaffold, segment{a: tops[i], b: tops[(i+1)%len(tops)], width: zoom})
		}
	}
	if site.Crane > 0 {
		foot := toward(lot[3], centre, 0.35)
		top := up(foot, (foot.Y-roofline+14*zoom)*site.Crane)
		reach := (lot[1].X - lot[3].X) * 0.7 * site.Crane
		crane = append(crane,
			segment{a: foot, b: top, width: 2 * zoom, pole: true},
			segment{a: top, b: city.Point{X: top.X + reach, Y: top.Y}, width: 1.5 * zoom},
			segment{a: top, b: city.Point{X: top.X - reach*0.3, Y: top.Y}, width: 1.5 * zoom},
		)
	}
	return fence, scaffold, crane
}

// lotCorners is a building's lot on screen: north, east, south, west.
func lotCorners(cam *city.Camera, r city.Rect) [4]city.Point {
	plot := r.Inset(city.Tile * 0.1)
	var corners [4]city.Point
	for i, p := range []city.Point{plot.Min, {X: plot.Max.X, Y: plot.Min.Y}, plot.Max, {X: plot.Min.X, Y: plot.Max.Y}} {
		corners[i] = cam.WorldToScreen(p)
	}
	return corners
}

// kitTop is how far above its ground point a piece's sprite reaches on
// screen: the roofline a build rises to.
func (g *Game) kitTop(cam *city.Camera, name string) float64 {
	if g.kits == nil {
		return 0
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return 0
	}
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok {
		return 0
	}
	return float64(sprite.Anchor.Y) * (cam.Zoom / atlas.Zoom)
}

// buildTint is a building under construction: dark until its lights come
// on, and faded by a cross-fade.
func buildTint(tint *ebiten.ColorScale, site city.Site) *ebiten.ColorScale {
	out := &ebiten.ColorScale{}
	if tint != nil {
		*out = *tint
	}
	lit := float32(0.55 + 0.45*site.Lights)
	out.Scale(lit, lit, lit, 1)
	out.ScaleAlpha(float32(site.Opacity))
	return out
}

// drawSite strokes a build's site in the order it stands: the fence
// first, then the scaffolding around the floors, then the crane over all.
func (g *Game) drawSite(screen *ebiten.Image, lot [4]city.Point, roofline float64, site city.Site, zoom float64) {
	fence, scaffold, crane := siteLines(lot, roofline, site, zoom)
	for _, set := range []struct {
		lines []segment
		color color.NRGBA
	}{{fence, colorFence}, {scaffold, colorScaffold}, {crane, colorSiteCrane}} {
		for _, s := range set.lines {
			vector.StrokeLine(screen, float32(s.a.X), float32(s.a.Y), float32(s.b.X), float32(s.b.Y), float32(max(1, s.width)), set.color, true)
		}
	}
}
