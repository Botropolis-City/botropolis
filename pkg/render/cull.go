package render

import (
	"botropolis.city/botropolis/pkg/city"
)

// At full zoom most of the city is off the frame, and every building in
// it was drawn each frame all the same: on 217 sessions at 1920×1200, 37
// were on screen and the other 180 filled the look pages, so those on
// screen were drawn straight (item 161). A building whose cull bound
// misses the frame is now not drawn at all.

// cullPad is how far past its look's bounds a building is taken to
// paint, as a share of its sprite's width: its lights' glow, a beacon,
// smoke rising.
const cullPad = 0.5

// culled is whether a building is left undrawn this frame: its cull
// bound misses the frame. Drawn straight, as the check against the
// standing city draws, nothing is culled.
func (g *Game) culled(cam *city.Camera, b *city.Building, selected, detailed bool, seconds float64, frame city.Rect) bool {
	if g.noStill {
		return false
	}
	if farOffScreen(cam, b.Rect, frame) {
		return true
	}
	g.boundsWorked++
	r, ok := g.cullBound(cam, b, selected, detailed, seconds)
	return ok && !r.Overlaps(frame)
}

// cullBound is the screen box a building paints inside, and whether it
// can be culled at all: one not drawn as steps is never culled.
func (g *Game) cullBound(cam *city.Camera, b *city.Building, selected, detailed bool, seconds float64) (city.Rect, bool) {
	f, ok := g.buildingFrame(cam, b, selected, detailed, seconds)
	if !ok {
		return city.Rect{}, false
	}
	r := g.lookBounds(cam, &f)
	return r.Inset(-cullPad * g.spriteRect(cam, &f).Width()), true
}

// farOffScreen is whether a building's footprint lands so far from the
// frame that nothing it paints can reach it, padded by the footprint's
// own width on the screen: a sprite is about as wide as its ground,
// paints half a width past it, and rises from it by a few widths at
// most. It answers before the building's look is worked out, which was
// done for every building in the city each frame to draw the few on
// screen (item 270).
func farOffScreen(cam *city.Camera, r city.Rect, frame city.Rect) bool {
	fp := footprintOnScreen(cam, cam.TurnRect(r))
	w := fp.Width()
	near := cullFrame(frame)
	near.Min.X -= w
	near.Max.X += w
	near.Min.Y -= w
	near.Max.Y += 4 * w
	return !fp.Overlaps(near)
}
