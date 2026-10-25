package render

import (
	"encoding/binary"
	"hash/maphash"
	"image"
	"slices"
)

// entry is one drawable of the frame as the dry pass recorded it:
// whether it moves, and the range of the night's light ops it noted.
type entry struct {
	inked
	moves  bool
	lights [2]int
	// active is whether it shows activity: a traveller — a car, a
	// drone, a boat — or a building animating for its session, its
	// beacon pulsing, its scaffold up, its smoke; not the fountain,
	// which sprays whatever the sessions do.
	active bool
	// key is what the drawable was drawn from, 0 for a drawable that
	// keeps no record; keeps whether its record can be kept for the next
	// frame, and world how many world draws it counted (item 212).
	key   uint64
	keeps bool
	world int
	jn    noteLog
}

// stillSet is the still drawables a layer was composed from, counted by
// what each paints and where. It holds no order: two stills the depth
// sort leaves unordered cannot cover one another, and their order
// shifts as movers come and go, which must not compose anything again
// (item 193).
type stillSet struct {
	marks, now map[uint64]stillMark
}

type stillMark struct {
	hash uint64
	box  image.Rectangle
	n    int
}

func newStillSet(entries []entry) *stillSet {
	s := &stillSet{marks: map[uint64]stillMark{}, now: map[uint64]stillMark{}}
	s.reset(entries)
	return s
}

// reset makes the set the still drawables of entries.
func (s *stillSet) reset(entries []entry) {
	clear(s.marks)
	count(s.marks, entries)
}

// changes appends to out the box of every still drawable that is in
// entries and not in the set, and of every one in the set and not in
// entries: where the layer must be composed again.
func (s *stillSet) changes(out []image.Rectangle, entries []entry) []image.Rectangle {
	clear(s.now)
	count(s.now, entries)
	for k, m := range s.now {
		if s.marks[k].n < m.n {
			out = append(out, m.box)
		}
	}
	added := len(out)
	for k, m := range s.marks {
		if s.now[k].n < m.n && !slices.Contains(out[:added], m.box) {
			out = append(out, m.box)
		}
	}
	return out
}

func count(marks map[uint64]stillMark, entries []entry) {
	for _, e := range entries {
		if e.moves {
			continue
		}
		k := markOf(e.hash, e.box)
		m := marks[k]
		m.hash, m.box, m.n = e.hash, e.box, m.n+1
		marks[k] = m
	}
}

// shift moves every drawable in the set by k, as a whole-pixel pan
// moves them on the frame.
func (s *stillSet) shift(k image.Point) {
	clear(s.now)
	for _, m := range s.marks {
		if m.box != everywhere {
			m.box = m.box.Add(k)
		}
		s.now[markOf(m.hash, m.box)] = m
	}
	s.marks, s.now = s.now, s.marks
}

// markOf is what a still drawable paints and where.
func markOf(hash uint64, box image.Rectangle) uint64 {
	var b [40]byte
	binary.LittleEndian.PutUint64(b[0:], hash)
	for i, v := range []int{box.Min.X, box.Min.Y, box.Max.X, box.Max.Y} {
		binary.LittleEndian.PutUint64(b[8+i*8:], uint64(int64(v)))
	}
	return maphash.Bytes(inkSeed, b[:])
}

// withoutActivity is entries without what shows activity, in place:
// the frame the frame test compares two cities by, with the same
// activity, none (item 193).
func withoutActivity(entries []entry) []entry {
	out := entries[:0]
	for _, e := range entries {
		if !e.active {
			out = append(out, e)
		}
	}
	return out
}
