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
	box image.Rectangle
	n   int
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
		k := markOf(e)
		m := marks[k]
		m.box, m.n = e.box, m.n+1
		marks[k] = m
	}
}

// markOf is what a still drawable paints and where.
func markOf(e entry) uint64 {
	var b [40]byte
	binary.LittleEndian.PutUint64(b[0:], e.hash)
	for i, v := range []int{e.box.Min.X, e.box.Min.Y, e.box.Max.X, e.box.Max.Y} {
		binary.LittleEndian.PutUint64(b[8+i*8:], uint64(int64(v)))
	}
	return maphash.Bytes(inkSeed, b[:])
}
