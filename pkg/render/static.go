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
// Only the layers painted before the sorted list live here. The sorted
// list's still half is composed over this one as the standing city, and
// what moves is drawn again over that from the ground up (item 193,
// stillframe.go).

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
	// hash is what the layer painted, by the dry pass (item 193).
	hash uint64
	// composes is how often it was painted this frame, for the frame log.
	composes int
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

// groundLayer is the city under the traffic this frame draws over: the
// game's, or its own for a frame drawn straight, so the check of the
// standing city keeps each from the other (item 193).
func (g *Game) groundLayer() *staticLayer {
	if g.noStill {
		return &g.straightStatic
	}
	return &g.static
}

// staticCity returns the composed city under the traffic, drawing it
// again only when its key has changed, and then only if what it would
// paint has.
func (g *Game) staticCity(c *city.City, cam *city.Camera, hover city.Hit, width, height float64) *ebiten.Image {
	st := g.groundLayer()
	key := g.keyFor(cam, hover, int(width), int(height))
	frames.note(st.ok && st.key == key && st.img != nil)
	if st.ok && st.key == key && st.img != nil {
		return st.img
	}
	if st.img == nil || st.img.Bounds().Dx() != int(width) || st.img.Bounds().Dy() != int(height) {
		if st.img != nil {
			st.img.Deallocate()
		}
		st.img = ebiten.NewImage(int(width), int(height))
		st.ok = false
	}
	// The layers under the traffic own hover targets too — a street, a
	// district floor, a tower beam. They are recorded while composing
	// and replayed every frame, which is sound for exactly as long as
	// the key holds, because the key is what fixes them on screen.
	g.mu.Lock()
	g.frameHits = g.frameHits[:0]
	g.mu.Unlock()
	if g.noStill {
		st.img.Clear()
		g.composeStatic(st.img, c, cam, hover, width, height)
		st.composes++
	} else {
		// A new key need not be a new picture: a snapshot rebuilds the
		// city whether or not anything in it looks different. The dry
		// pass says whether it does, and only then is it painted.
		l := g.recorder()
		l.begin()
		g.composeStatic(l.dry, c, cam, hover, width, height)
		in := l.end()
		if !st.ok || in.hash != st.hash {
			st.img.Clear()
			f := g.newFadePlay(st.img)
			l.play(st.img, in, f)
			f.done()
			st.hash = in.hash
			st.composes++
		}
	}
	g.mu.Lock()
	st.hits = append(st.hits[:0], g.frameHits...)
	g.mu.Unlock()
	st.key, st.ok = key, true
	return st.img
}

// composeStatic paints the layers that sit under the sorted list.
func (g *Game) composeStatic(dst *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit, width, height float64) {
	g.isoGround(dst, cam, c, width, height)
	// The roads' tags are laid out with the other labels each frame,
	// over the world, rather than painted here under it (item 102).
	g.streets(dst, c, cam, hover, false)
	g.beams(dst, c, cam, hover)
	for _, d := range c.Districts {
		g.isoDistrict(dst, cam, d, hover.District == d)
	}
	g.drivewayPads(dst, c, cam, false)
	g.fadeAll(&dst, func() { g.isoPlaza(dst, cam, c) })
	g.camps(dst, c, cam)
}
