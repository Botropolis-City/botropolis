package render

import "slices"

// sizeWindow is how many frames running a buffer must use under a
// quarter of what it holds before it is made smaller, so a city that
// swings between big frames and small ones keeps room for the big; and
// sizeFloor the least a buffer is made (item 212).
const (
	sizeWindow = 8
	sizeFloor  = 64
)

// sizing is the most one of the record's buffers used in each of the
// last sizeWindow frames.
type sizing struct {
	used [sizeWindow]int
	n    int
}

// peak notes what a frame used and is the most used over the window,
// or -1 until the window has filled.
func (s *sizing) peak(used int) int {
	s.used[s.n%sizeWindow] = used
	s.n++
	if s.n < sizeWindow {
		return -1
	}
	return slices.Max(s.used[:])
}

// fitted is b emptied, made twice peak when it holds over four times
// that, and whether it was.
func fitted[T any](b []T, peak int) ([]T, bool) {
	size := max(2*peak, sizeFloor)
	if peak < 0 || cap(b) <= 2*size {
		return b[:0], false
	}
	return make([]T, 0, size), true
}

// fitRecord gives back what the buffers l records into hold beyond what
// the frames have used, from what the frame just put aside in k used.
// Both halves are fitted, each as it is given to be recorded into.
func (g *Game) fitRecord(l *drawList, k *keptRecord) {
	s := &g.still
	shrank := 0
	fit := func(i int, used int, f func(peak int) bool) {
		if f(s.sizes[i].peak(used)) {
			shrank++
		}
	}
	fit(0, len(k.cmds), func(p int) (ok bool) { l.cmds, ok = fitted(l.cmds, p); return })
	fit(1, len(k.vertices), func(p int) (ok bool) { l.vertices, ok = fitted(l.vertices, p); return })
	fit(2, len(k.indices), func(p int) (ok bool) { l.indices, ok = fitted(l.indices, p); return })
	fit(3, len(k.points), func(p int) (ok bool) { l.points, ok = fitted(l.points, p); return })
	fit(4, len(k.imgOps), func(p int) (ok bool) { l.imgOps, ok = fitted(l.imgOps, p); return })
	fit(5, len(k.cmOps), func(p int) (ok bool) { l.cmOps, ok = fitted(l.cmOps, p); return })
	fit(6, len(k.triOps), func(p int) (ok bool) { l.triOps, ok = fitted(l.triOps, p); return })
	fit(7, len(k.pathOps), func(p int) (ok bool) { l.pathOps, ok = fitted(l.pathOps, p); return })
	fit(8, len(k.textOps), func(p int) (ok bool) { l.textOps, ok = fitted(l.textOps, p); return })
	fit(9, len(k.fills), func(p int) (ok bool) { l.fills, ok = fitted(l.fills, p); return })
	if shrank > 0 {
		clear(l.cmds[:cap(l.cmds)])
	}
	s.shrinks += shrank
}
