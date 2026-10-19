package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 108, Aria 2026-10-18: a road's tag steps off a car rather than
// covering it. Only the tag does: a plate or a badge that hopped aside
// whenever a car drove under it would jump all day.
func TestLabelsStepOffCars(t *testing.T) {
	const gap = 2.0
	car := mapLabel{Kind: labelCar, Rect: city.RectAt(100, 100, 30, 20), Anchor: city.Point{X: 115, Y: 110}}
	over := city.RectAt(95, 105, 50, 12)

	for _, c := range []struct {
		kind  labelKind
		steps bool
	}{{labelTag, true}, {labelBadge, false}, {labelPlate, false}, {labelStation, false}} {
		t.Run("when a "+c.kind.String()+" would stand over a car", func(t *testing.T) {
			out := layoutLabels([]mapLabel{car, {Kind: c.kind, Rect: over, Anchor: over.Center()}}, gap)

			if c.steps {
				t.Run("it should step off the car", func(t *testing.T) {
					assert.True(t, out[1].Shown && !out[1].At.Overlaps(car.Rect), "laid at %v", out[1].At)
				})
			} else {
				t.Run("it should stay where it stands", func(t *testing.T) {
					assert.Equal(t, laidLabel{At: over, Shown: true}, out[1])
				})
			}
		})
	}

	t.Run("when a car is laid out", func(t *testing.T) {
		out := layoutLabels([]mapLabel{car}, gap)

		t.Run("it should draw nothing for it", func(t *testing.T) {
			assert.False(t, out[0].Shown)
		})
	})
}
