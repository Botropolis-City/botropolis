package render

import (
	"hash/maphash"
	"image"

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
	projection city.Projection
	// hover moves the highlight on a street, a district and a tower
	// beam, all of which are painted into this layer.
	hoverDistrict *city.District
	hoverTower    *city.Tower
	// hoverRoad and hoverBeam are the road and the beam under the
	// pointer, each painted lit (item 207).
	hoverRoad     *city.RoadLine
	hoverBeamFrom *city.Building
	hoverBeamTo   *city.Tower
	// hoverCar is the road of the car under the pointer, which recedes
	// every other lot and road (item 108).
	hoverCar *city.RoadLine
	// roads is the session whose roads are shown (item 88).
	roads string
	// lit is what the highlight lights and fades on the ground: the
	// roads it holds, the districts it fades and whether it recedes the
	// ground, asked as composeStatic asks them (item 207).
	lit uint64
	// atlas is the kit atlas the ground is drawn from, which changes as
	// its pages finish going up (item 207).
	atlas *kitAtlas
}

// staticLayer is the composed image and the key it was composed for.
type staticLayer struct {
	img  *ebiten.Image
	key  layerKey
	hits []spriteHit
	ok   bool
	// whole counts how often the layer was painted whole, set holds
	// what each of its draws painted and where, and redo is where it
	// was painted again this frame, for the standing city over it to
	// compose again there too (item 193).
	whole         uint64
	set           *stillSet
	marks         []entry
	changed, redo []image.Rectangle
	// composes and shifts are how often it was painted and shifted this
	// frame, for the frame log, and origin the camera's origin it was
	// painted at.
	composes, shifts int
	origin           city.Point
}

// keyFor is the key this frame would compose under.
func (g *Game) keyFor(cam *city.Camera, hover city.Hit, width, height int) layerKey {
	key := layerKey{
		generation:    g.scene.Generation(),
		offset:        cam.Offset,
		zoom:          cam.Zoom,
		heading:       cam.Heading,
		view:          g.scene.View(),
		width:         width,
		height:        height,
		night:         g.scene.City().Night,
		projection:    cam.Projection,
		hoverDistrict: hover.District,
		hoverTower:    hover.Tower,
		hoverRoad:     hover.Road,
		hoverCar:      carRoad(hover),
		roads:         g.scene.RoadsShown(),
		lit:           g.litOnGround(g.scene.City()),
	}
	if hover.Beam != nil {
		key.hoverBeamFrom, key.hoverBeamTo = hover.Beam.Building, hover.Beam.Tower
	}
	if g.kits != nil {
		key.atlas = g.kits.pick(cam.Zoom)
	}
	return key
}

// litOnGround is a fingerprint of what the highlight lights and fades
// in the ground layer, asked of it as composeStatic asks.
func (g *Game) litOnGround(c *city.City) uint64 {
	if !g.lit.Active {
		return 0
	}
	var h maphash.Hash
	h.SetSeed(inkSeed)
	var b [1]byte
	put := func(v bool) {
		b[0] = 0
		if v {
			b[0] = 1
		}
		h.Write(b[:])
	}
	put(g.lit.RecedesGround())
	for i := range c.Roads {
		put(g.lit.HasRoad(&c.Roads[i]))
	}
	for _, st := range c.Streets {
		put(g.lit.HasRoad(st.Road))
	}
	for _, d := range c.Districts {
		put(g.lit.FadesDistrict(d))
	}
	return h.Sum64() | 1
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
		st.img = sizedImage(nil, image.Pt(int(width), int(height)))
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
		// Each of its draws is compared, as the standing city's drawables
		// are: a pan by whole pixels moves the layer and paints what it
		// uncovers, and a draw that changed is painted again where it
		// was and where it is, and nowhere else.
		st.marks = st.marks[:0]
		for _, c := range l.cmds[in.from:in.to] {
			st.marks = append(st.marks, entry{inked: inked{hash: c.hash, box: c.box}})
		}
		if st.set == nil {
			st.set = newStillSet(nil)
		}
		whole := !st.ok
		var shift image.Point
		if !whole && l.origin != st.origin {
			k, ok := wholeShift(st.origin, l.origin, st.img.Bounds())
			whole, shift = !ok, k
		}
		frame := st.img.Bounds()
		st.changed = st.changed[:0]
		if !whole {
			if shift != (image.Point{}) {
				g.shiftLayer(&st.img, shift)
				st.set.shift(shift)
				st.changed = append(st.changed, exposed(frame, shift)...)
				st.shifts++
			}
			st.changed = st.set.changes(st.changed, st.marks)
			st.redo = dirtyRects(st.redo[:0], st.changed, frame)
			whole = area(st.redo) > frame.Dx()*frame.Dy()/2
		}
		if whole {
			st.img.Clear()
			f := g.newFadePlay(st.img)
			l.play(st.img, in, f)
			f.done()
			st.whole++
			st.composes++
			st.redo = st.redo[:0]
		}
		for _, r := range st.redo {
			sub := st.img.SubImage(r).(*ebiten.Image)
			sub.Clear()
			f := g.newFadePlay(sub)
			l.playIn(sub, in, f, r)
			f.done()
		}
		st.set.reset(st.marks)
		st.origin = l.origin
	}
	g.mu.Lock()
	st.hits = append(st.hits[:0], g.frameHits...)
	g.mu.Unlock()
	st.key, st.ok = key, true
	return st.img
}

// composeStatic paints the layers that sit under the sorted list.
// Anything it reads must be in layerKey, or the layer kept shows what
// it read before: TestTheGroundKey walks the pointer, the view and the
// selection at random to catch an input left out (item 207).
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
