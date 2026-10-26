package render

import (
	"math"
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
	// labelLandmark names the power plant, the library or city hall,
	// over the top of its sprite, stepping aside for signs and badges
	// (item 99).
	labelLandmark
	// labelTag is a road's traffic tag, the least of them.
	labelTag
	// labelCar is a car on the street: not a label but a thing a road's
	// tag steps off, never drawn and never moved (item 108).
	labelCar
)

func (k labelKind) String() string {
	return [...]string{"sign", "plate", "caption", "station label", "badge", "landmark", "tag", "car"}[k]
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
	laid, _ := layoutLabelsCounted(labels, gap)
	return laid
}

func layoutLabelsCounted(labels []mapLabel, gap float64) ([]laidLabel, int) {
	checks := 0
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
	var taken, cars rectGrid
	for _, l := range labels {
		if l.Kind == labelCar {
			cars.add(l.Rect)
		}
	}
	free := func(kind labelKind, r city.Rect) bool {
		if taken.hits(r, &checks) {
			return false
		}
		return kind != labelTag || !cars.hits(r, &checks)
	}
	for _, i := range order {
		l := labels[i]
		if l.Kind == labelCar {
			continue
		}
		if l.Kind == labelSign {
			out[i] = laidLabel{At: l.Rect, Shown: true}
			taken.add(l.Rect)
			continue
		}
		dx, dy := l.Rect.Width()+gap, l.Rect.Height()+gap
		for s, step := range labelSteps {
			at := city.RectAt(l.Rect.Min.X+step[0]*dx, l.Rect.Min.Y+step[1]*dy, l.Rect.Width(), l.Rect.Height())
			if free(l.Kind, at) {
				out[i] = laidLabel{At: at, Shown: true, Leader: s >= labelNearSteps}
				taken.add(at)
				break
			}
		}
	}
	return out, checks
}

// rectGrid holds the rectangles already placed, filed by the grid cells
// they cover, so a label is tested only against those near it: the
// layout compared each label with every one placed before it, every
// frame, which at a plate a district was quadratic (item 270). One too
// large to file cell by cell is tested against everything.
type rectGrid struct {
	rects    []city.Rect
	cells    map[[2]int][]int32
	oversize []int32
}

const (
	rectGridCell = 64.0
	rectGridMost = 64
)

func (g *rectGrid) span(r city.Rect) (x0, y0, x1, y1 int, ok bool) {
	fx0, fy0 := math.Floor(r.Min.X/rectGridCell), math.Floor(r.Min.Y/rectGridCell)
	fx1, fy1 := math.Floor(r.Max.X/rectGridCell), math.Floor(r.Max.Y/rectGridCell)
	if !(fx1-fx0 < rectGridMost && fy1-fy0 < rectGridMost) {
		return 0, 0, 0, 0, false
	}
	return int(fx0), int(fy0), int(fx1), int(fy1), true
}

func (g *rectGrid) add(r city.Rect) {
	i := int32(len(g.rects))
	g.rects = append(g.rects, r)
	x0, y0, x1, y1, ok := g.span(r)
	if !ok {
		g.oversize = append(g.oversize, i)
		return
	}
	if g.cells == nil {
		g.cells = map[[2]int][]int32{}
	}
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			g.cells[[2]int{x, y}] = append(g.cells[[2]int{x, y}], i)
		}
	}
}

// hits is whether r overlaps a rectangle in the grid, counting each
// rectangle it tests in checks.
func (g *rectGrid) hits(r city.Rect, checks *int) bool {
	for _, i := range g.oversize {
		*checks++
		if r.Overlaps(g.rects[i]) {
			return true
		}
	}
	x0, y0, x1, y1, ok := g.span(r)
	if !ok {
		for _, t := range g.rects {
			*checks++
			if r.Overlaps(t) {
				return true
			}
		}
		return false
	}
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for _, i := range g.cells[[2]int{x, y}] {
				*checks++
				if r.Overlaps(g.rects[i]) {
					return true
				}
			}
		}
	}
	return false
}
