package render

import "image"

// dirtyRects merges the boxes of what moves this frame into the
// rectangles drawn again, appended to out: boxes that meet or touch
// become one, until no two rectangles meet, each clipped to the frame
// (item 193). Two rectangles that overlapped would play a draw twice
// over the pixels they share, blending its soft rim twice.
func dirtyRects(out []image.Rectangle, boxes []image.Rectangle, frame image.Rectangle) []image.Rectangle {
	start := len(out)
	for _, b := range boxes {
		b = b.Intersect(frame)
		if b.Empty() {
			continue
		}
		for {
			merged := false
			for i := start; i < len(out); i++ {
				if meet(out[i], b) {
					b = b.Union(out[i])
					out[i] = out[len(out)-1]
					out = out[:len(out)-1]
					merged = true
					break
				}
			}
			if !merged {
				break
			}
		}
		out = append(out, b)
	}
	return out
}

// meet is whether two rectangles overlap or share an edge.
func meet(a, b image.Rectangle) bool {
	return a.Min.X <= b.Max.X && b.Min.X <= a.Max.X && a.Min.Y <= b.Max.Y && b.Min.Y <= a.Max.Y
}

// area is how many pixels rectangles cover, counting any overlap twice.
func area(rs []image.Rectangle) int {
	n := 0
	for _, r := range rs {
		n += r.Dx() * r.Dy()
	}
	return n
}
