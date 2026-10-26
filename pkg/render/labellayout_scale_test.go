package render

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// pairwiseLayout is the label layout as item 102 wrote it, which
// compared each label with every one placed before it: the oracle the
// layout must agree with.
func pairwiseLayout(labels []mapLabel, gap float64) []laidLabel {
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
	var taken, cars []city.Rect
	for _, l := range labels {
		if l.Kind == labelCar {
			cars = append(cars, l.Rect)
		}
	}
	free := func(kind labelKind, r city.Rect) bool {
		for _, t := range taken {
			if r.Overlaps(t) {
				return false
			}
		}
		if kind == labelTag {
			for _, c := range cars {
				if r.Overlaps(c) {
					return false
				}
			}
		}
		return true
	}
	for _, i := range order {
		l := labels[i]
		if l.Kind == labelCar {
			continue
		}
		if l.Kind == labelSign {
			out[i] = laidLabel{At: l.Rect, Shown: true}
			taken = append(taken, l.Rect)
			continue
		}
		dx, dy := l.Rect.Width()+gap, l.Rect.Height()+gap
		for s, step := range labelSteps {
			at := city.RectAt(l.Rect.Min.X+step[0]*dx, l.Rect.Min.Y+step[1]*dy, l.Rect.Width(), l.Rect.Height())
			if free(l.Kind, at) {
				out[i] = laidLabel{At: at, Shown: true, Leader: s >= labelNearSteps}
				taken = append(taken, at)
				break
			}
		}
	}
	return out
}

// Item 270: a city of 600 districts asks for a plate a district, and the
// layout compared each with every label already placed, every frame.
func TestLayoutLabelsAtScale(t *testing.T) {
	const n, gap = 3000, 2.0
	t.Run("when 3,000 labels are scattered across a wide map", func(t *testing.T) {
		r := rand.New(rand.NewSource(270))
		kinds := []labelKind{labelSign, labelPlate, labelStation, labelBadge, labelLandmark, labelTag, labelCar}
		labels := make([]mapLabel, n)
		for i := range labels {
			x, y := r.Float64()*6000, r.Float64()*3000
			w, h := 30+r.Float64()*90, 10+r.Float64()*8
			labels[i] = mapLabel{Kind: kinds[r.Intn(len(kinds))], Rect: city.RectAt(x, y, w, h), Anchor: city.Point{X: x + w/2, Y: y + h}}
		}
		laid, checks := layoutLabelsCounted(labels, gap)

		t.Run("it should place every label where the pairwise layout places it", func(t *testing.T) {
			assert.Equal(t, pairwiseLayout(labels, gap), laid)
		})

		t.Run("it should compare each label only with its neighbours", func(t *testing.T) {
			assert.Less(t, checks, 50*n)
		})
	})
}
