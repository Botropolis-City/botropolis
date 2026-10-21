package render

import (
	"image"
	"reflect"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

// streamPlay is one draw of the record played into one rectangle.
type streamPlay struct {
	cmd, rect int
}

// stream is the order the rectangles something moves in are drawn again
// in, appended to out: every drawable whose box meets one, in the frame's
// order, and each of its draws into every rectangle the draw meets in
// turn. The rectangles do not overlap, so the order across them is free,
// and playing a draw into all of them together lets the same draw merge
// into one, where a rectangle at a time would start each run again
// (item 193).
func stream(out []streamPlay, entries []entry, box func(cmd int) image.Rectangle, rects []image.Rectangle) []streamPlay {
	for _, e := range entries {
		meets := false
		for _, r := range rects {
			if e.box.Overlaps(r) {
				meets = true
				break
			}
		}
		if !meets {
			continue
		}
		for c := e.from; c < e.to; c++ {
			b := box(c)
			for i, r := range rects {
				if b.Overlaps(r) && e.box.Overlaps(r) {
					out = append(out, streamPlay{cmd: c, rect: i})
				}
			}
		}
	}
	return out
}

// batch reorders plays, appended to out, so draws from one source go
// down together: it takes the earliest draw whose source is the one now
// going down and that passes no draw it overlaps in its rectangle, and
// when there is none, the earliest draw that passes none at all. Draws
// that do not overlap touch different pixels, so their order is free;
// draws that do keep theirs, and so the frame is the same (item 193).
func batch(out []streamPlay, plays []streamPlay, box func(cmd int) image.Rectangle, src func(cmd int) int) []streamPlay {
	n := len(plays)
	if n == 0 {
		return out
	}
	// waits[j] counts the earlier draws into j's rectangle that j
	// overlaps and that have not gone down; after lists whom each
	// releases.
	waits := make([]int, n)
	after := make([][]int, n)
	for j := range plays {
		bj := box(plays[j].cmd)
		for i := 0; i < j; i++ {
			if plays[i].rect == plays[j].rect && box(plays[i].cmd).Overlaps(bj) {
				waits[j]++
				after[i] = append(after[i], j)
			}
		}
	}
	done := make([]bool, n)
	current, have := 0, false
	for left := n; left > 0; left-- {
		pick := -1
		for j := range plays {
			if done[j] || waits[j] > 0 {
				continue
			}
			if have && src(plays[j].cmd) == current {
				pick = j
				break
			}
			if pick < 0 {
				pick = j
				if !have {
					break
				}
			}
		}
		done[pick] = true
		current, have = src(plays[pick].cmd), true
		for _, j := range after[pick] {
			waits[j]--
		}
		out = append(out, plays[pick])
	}
	return out
}

// batchSource is what Ebitengine merges draws on: the page they read,
// how they filter and blend, and whether they go through a colour
// matrix. Words are told apart by their face.
type batchSource struct {
	page   uintptr
	kind   cmdKind
	filter ebiten.Filter
	blend  ebiten.Blend
	face   unsafe.Pointer
}

// batchKey is a number for the source a recorded draw reads, the same
// for every draw Ebitengine could merge it with.
func (l *drawList) batchKey(i int) int {
	c := &l.cmds[i]
	k := batchSource{kind: c.kind}
	switch c.kind {
	case cmdImage:
		k.page, k.filter, k.blend = pageOf(c.src), c.img.Filter, c.img.Blend
	case cmdColorM:
		k.page, k.filter, k.blend = pageOf(c.src), c.cmImg.Filter, c.cmImg.Blend
	case cmdMesh:
		k.page, k.filter, k.blend = pageOf(c.src), c.tri.Filter, c.tri.Blend
	case cmdWords:
		k.face = faceID(c.face)
	}
	if l.batchKeys == nil {
		l.batchKeys = map[batchSource]int{}
	}
	id, ok := l.batchKeys[k]
	if !ok {
		id = len(l.batchKeys)
		l.batchKeys[k] = id
	}
	return id
}

// pageOf is the image a sub-image was cut from, or the image itself: the
// texture a draw reads. Ebitengine keeps it unexported as original; if it
// stops, draws are only grouped less well.
func pageOf(img *ebiten.Image) uintptr {
	if img == nil {
		return 0
	}
	if f := reflect.ValueOf(img).Elem().FieldByName("original"); f.IsValid() && !f.IsNil() {
		return f.Pointer()
	}
	return uintptr(unsafe.Pointer(img))
}
