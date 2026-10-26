package render

import (
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// Every tree, lamp, sign and monument went into the depth sort each
// frame, on screen or not: a city of 900 districts sorted all of them
// to draw the few in view (item 270). Buildings are culled by their
// look's bounds already (item 161); everything else is culled here, by
// where its footprint lands on the screen.
//
// A sprite rises from its footprint, so a footprint below the frame can
// still reach into it and one above it cannot: the frame is padded far
// below, a little above, and to each side for what stands wider than
// its ground.

// cullBelow is the least a footprint may stand below the frame and still
// be kept: taller than any sprite the city draws at the closest zoom.
const cullBelow = 800.0

func cullFrame(frame city.Rect) city.Rect {
	side := max(200, frame.Width()/4)
	return city.Rect{
		Min: city.Point{X: frame.Min.X - side, Y: frame.Min.Y - frame.Height()/10},
		Max: city.Point{X: frame.Max.X + side, Y: frame.Max.Y + max(cullBelow, frame.Height())},
	}
}

// onScreen is the drawables whose footprints land in or near the frame,
// in their order, in items' own storage.
func onScreen(cam *city.Camera, items []drawable, frame city.Rect) []drawable {
	near := cullFrame(frame)
	kept := items[:0]
	for _, d := range items {
		if footprintOnScreen(cam, d.turned).Overlaps(near) {
			kept = append(kept, d)
		}
	}
	return kept
}

// footprintOnScreen is the screen box around a footprint already turned
// into the camera's frame.
func footprintOnScreen(cam *city.Camera, t city.Rect) city.Rect {
	lo := city.Point{X: math.Inf(1), Y: math.Inf(1)}
	hi := city.Point{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, c := range [4]city.Point{t.Min, {X: t.Max.X, Y: t.Min.Y}, t.Max, {X: t.Min.X, Y: t.Max.Y}} {
		p := cam.Projection.Apply(c).Add(cam.Offset).Scale(cam.Zoom)
		lo.X, lo.Y = math.Min(lo.X, p.X), math.Min(lo.Y, p.Y)
		hi.X, hi.Y = math.Max(hi.X, p.X), math.Max(hi.Y, p.Y)
	}
	return city.Rect{Min: lo, Max: hi}
}

// reachable is the labels that can stand somewhere on the frame, in
// their order, in labels' own storage: a label furthest out steps twice
// its size and the gap from where it asks to stand, so one further than
// that from the frame is never seen (item 270). A car is kept, since a
// tag steps off it.
func reachable(labels []pendingLabel, frame city.Rect, gap float64) []pendingLabel {
	kept := labels[:0]
	for _, l := range labels {
		r := l.Rect
		reach := r.Inset(-2 * (max(r.Width(), r.Height()) + gap))
		if l.Kind == labelCar || reach.Overlaps(frame) {
			kept = append(kept, l)
		}
	}
	return kept
}
