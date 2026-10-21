package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"

	"botropolis.city/botropolis/pkg/city"
)

// The landmarks light their own windows at night, as a session's
// building does (item 166): the library and City Hall all night, the
// power plant while any session is working, faded as the scene fades it
// (Aria, 2026-10-20, item 185).

// landmarkOn is how far a landmark's windows are lit tonight, given how
// far the plant's are: none by day.
func landmarkOn(c *city.City, kit string, plant float64) float64 {
	switch {
	case !c.Night:
		return 0
	case kit == kitPlant:
		return plant
	}
	return 1
}

// landmarkTriangles is a landmark's panes, lit as far as on, as
// triangles over its atlas page, where the landmark stands on its lot.
func (g *Game) landmarkTriangles(cam *city.Camera, lot city.Rect, kit string, on float64) ([]ebiten.Vertex, []uint16, *ebiten.Image) {
	if g.kits == nil || on <= 0 {
		return nil, nil, nil
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return nil, nil, nil
	}
	sprite, ok := atlas.Sprite(kit, cam.Heading-landmarkTurn)
	if !ok || sprite.Page >= len(atlas.pages) || len(sprite.Panes) == 0 {
		return nil, nil, nil
	}
	all := func(int) bool { return true }
	vertices, indices := paneVertices(sprite, cam.WorldToScreen(lot.Center()), cam.Zoom/atlas.Zoom, 1, min(1, on), all)
	return vertices, indices, atlas.pages[sprite.Page]
}

// lightLandmark notes a landmark's lit windows, once it is drawn and
// before anything in front of it is.
func (g *Game) lightLandmark(cam *city.Camera, lot city.Rect, kit string, on float64) {
	if !g.lightsOn {
		return
	}
	vertices, indices, page := g.landmarkTriangles(cam, lot, kit, on)
	if page == nil {
		return
	}
	g.noteLightsTinted(vertices, indices, page, true)
	g.landmarkDraws++
}

// The landmarks' glass is the kit's blue, so lit at its own brightness it
// reads cool beside the city's warm windows; at night it is mixed a
// little toward them (Aria, 2026-10-20: "Tint them a little yellow at
// night only"). By day nothing is laid over it.
var (
	colorWarmGlass = color.NRGBA{0xf5, 0xcd, 0x78, 0xff}
	landmarkWarmth = 0.3
	warmGlass      = func() colorm.ColorM {
		var m colorm.ColorM
		k := landmarkWarmth
		m.Scale(1-k, 1-k, 1-k, 1)
		m.Translate(k*float64(colorWarmGlass.R)/0xff, k*float64(colorWarmGlass.G)/0xff, k*float64(colorWarmGlass.B)/0xff, 0)
		return m
	}()
	warmOptions = &colorm.DrawTrianglesOptions{Filter: ebiten.FilterLinear, ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}
)
