package render

import (
	"sort"

	"botropolis.city/botropolis/pkg/city"
)

// Labels on the map are laid out together after the world is drawn, so
// that none covers another (item 102). A district sign, or the plate
// that stands in for it, has the right of way and never moves; a
// station label comes next, because it names a whole line; a badge
// last, the front-most first. A label that would cover one already
// placed steps aside or down, then further out on a leader back to
// what it names, and is left off when there is no room at all.
type labelKind int

const (
	labelSign labelKind = iota
	// labelPlate is the floating plate that names a district whose sign
	// is too small to read, or is under the pointer: a label, so z hides
	// it, placed before every other label and stepping aside only for a
	// sign.
	labelPlate
	// labelCaption is a first sighting's caption, saying what a kind of
	// thing is the first time it is on screen: placed like a plate.
	labelCaption
	labelStation
	labelBadge
	// labelTag is a road's traffic tag, the least of them.
	labelTag
)

func (k labelKind) String() string {
	return [...]string{"sign", "plate", "caption", "station label", "badge", "tag"}[k]
}

// mapLabel is a label asking for a place: Rect is where it would stand
// on its own, Anchor the point on the map it names.
type mapLabel struct {
	Kind   labelKind
	Rect   city.Rect
	Anchor city.Point
}

// laidLabel is where a label was put, whether it is drawn at all, and
// whether it stands far enough from its anchor to need a leader.
type laidLabel struct {
	At     city.Rect
	Shown  bool
	Leader bool
}

// labelSteps are the offsets a label tries, in label widths and
// heights plus the gap: its own place, then a step down, aside or up,
// then the same twice as far out, on a leader.
var labelSteps = [][2]float64{
	{0, 0},
	{0, 1}, {1, 0}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1},
	{0, 2}, {2, 0}, {-2, 0}, {0, -2}, {2, 2}, {-2, 2},
}

const labelNearSteps = 7

func layoutLabels(labels []mapLabel, gap float64) []laidLabel {
	order := make([]int, len(labels))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		la, lb := labels[order[a]], labels[order[b]]
		if la.Kind != lb.Kind {
			return la.Kind < lb.Kind
		}
		return la.Anchor.Y > lb.Anchor.Y
	})
	out := make([]laidLabel, len(labels))
	var taken []city.Rect
	free := func(r city.Rect) bool {
		for _, t := range taken {
			if r.Overlaps(t) {
				return false
			}
		}
		return true
	}
	for _, i := range order {
		l := labels[i]
		if l.Kind == labelSign {
			out[i] = laidLabel{At: l.Rect, Shown: true}
			taken = append(taken, l.Rect)
			continue
		}
		dx, dy := l.Rect.Width()+gap, l.Rect.Height()+gap
		for s, step := range labelSteps {
			at := city.RectAt(l.Rect.Min.X+step[0]*dx, l.Rect.Min.Y+step[1]*dy, l.Rect.Width(), l.Rect.Height())
			if free(at) {
				out[i] = laidLabel{At: at, Shown: true, Leader: s >= labelNearSteps}
				taken = append(taken, at)
				break
			}
		}
	}
	return out
}
