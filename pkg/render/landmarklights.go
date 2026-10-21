package render

import (
	"github.com/hajimehoshi/ebiten/v2"

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
	g.noteLights(vertices, indices, page)
	g.landmarkDraws++
}
