package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// The city under the traffic. Ground, avenues, tower beams, district
// floors, the plaza and the camp ties do not change between frames
// unless the city, the camera or the view does — and they are most of
// the drawing. Composed once into an offscreen image and blitted, they
// cost one draw call a frame instead of thousands.
//
// The power lines are not among them although they are painted in the
// same place: their sparks travel with the clock, so they would freeze.
//
// Only the layers painted before the sorted list live here, and that is
// deliberate rather than a first cut: everything in the sorted list is
// interleaved with the things that move, so lifting it out would paint
// a car over the building it is driving behind. Caching that half needs
// the movers to redraw the static pieces they pass in front of, which
// is its own piece of work.

// layerKey is everything the static layer depends on. When it changes
// the layer is composed again; while it holds, the layer is a blit.
type layerKey struct {
	generation int
	offset     city.Point
	zoom       float64
	heading    int
	view       city.View
	width      int
	height     int
	night      bool
	labels     bool
	projection city.Projection
	// hover moves the highlight on a street, a district and a tower
	// beam, all of which are painted into this layer.
	hoverDistrict *city.District
	hoverTower    *city.Tower
	hoverLine     *city.PowerLine
	// hoverCar is the road of the car under the pointer, which recedes
	// every other lot and road (item 108).
	hoverCar *city.RoadLine
	// roads is the session whose roads are shown (item 88).
	roads string
}

// staticLayer is the composed image and the key it was composed for.
type staticLayer struct {
	img  *ebiten.Image
	key  layerKey
	hits []spriteHit
	ok   bool
}

// keyFor is the key this frame would compose under.
func (g *Game) keyFor(cam *city.Camera, hover city.Hit, width, height int) layerKey {
	return layerKey{
		generation:    g.scene.Generation(),
		offset:        cam.Offset,
		zoom:          cam.Zoom,
		heading:       cam.Heading,
		view:          g.scene.View(),
		width:         width,
		height:        height,
		night:         g.scene.City().Night,
		labels:        g.labelsVisible(),
		projection:    cam.Projection,
		hoverDistrict: hover.District,
		hoverTower:    hover.Tower,
		hoverLine:     hover.Line,
		hoverCar:      carRoad(hover),
		roads:         g.scene.RoadsShown(),
	}
}

// staticCity returns the composed city under the traffic, drawing it
// again only when its key has changed.
func (g *Game) staticCity(c *city.City, cam *city.Camera, hover city.Hit, width, height float64) *ebiten.Image {
	key := g.keyFor(cam, hover, int(width), int(height))
	frames.note(g.static.ok && g.static.key == key && g.static.img != nil)
	if g.static.ok && g.static.key == key && g.static.img != nil {
		return g.static.img
	}
	if g.static.img == nil || g.static.img.Bounds().Dx() != int(width) || g.static.img.Bounds().Dy() != int(height) {
		if g.static.img != nil {
			g.static.img.Deallocate()
		}
		g.static.img = ebiten.NewImage(int(width), int(height))
	}
	g.static.img.Clear()
	// The layers under the traffic own hover targets too — a street, a
	// district floor, a tower beam. They are recorded while composing
	// and replayed every frame, which is sound for exactly as long as
	// the key holds, because the key is what fixes them on screen.
	g.mu.Lock()
	g.frameHits = g.frameHits[:0]
	g.mu.Unlock()
	g.composeStatic(g.static.img, c, cam, hover, width, height)
	g.mu.Lock()
	g.static.hits = append(g.static.hits[:0], g.frameHits...)
	g.mu.Unlock()
	g.static.key, g.static.ok = key, true
	return g.static.img
}

// composeStatic paints the layers that sit under the sorted list.
func (g *Game) composeStatic(dst *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit, width, height float64) {
	labels := g.labelsVisible()
	g.isoGround(dst, cam, c, width, height)
	g.streets(dst, c, cam, hover, labels)
	g.beams(dst, c, cam, hover)
	for _, d := range c.Districts {
		g.isoDistrict(dst, cam, d, hover.District == d)
	}
	g.drivewayPads(dst, c, cam, false)
	g.isoPlaza(dst, cam, c)
	g.camps(dst, c, cam)
}
