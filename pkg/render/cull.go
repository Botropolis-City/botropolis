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
