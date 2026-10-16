package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: ambient motion runs at the level's pace and attention at the
// clock's, and the clocks are stepped by the caller, never by the wall.
func TestMotionClock(t *testing.T) {
	start := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	for _, level := range city.LivelinessLevels {
		t.Run("when the level is "+level.String(), func(t *testing.T) {
			t.Run("and a second passes", func(t *testing.T) {
				m := newMotionClock(start)
				m.step(start.Add(time.Second), level)

				t.Run("it should advance the attention clock by a second", func(t *testing.T) {
					assert.InDelta(t, 1.0, m.attention, 1e-9)
				})

				t.Run("it should advance the ambient clock by the level's pace", func(t *testing.T) {
					assert.InDelta(t, level.Pace(), m.ambient, 1e-9)
				})
			})
		})
	}

	t.Run("when the level changes partway", func(t *testing.T) {
		m := newMotionClock(start)
		m.step(start.Add(2*time.Second), city.Lively)
		before := m.ambient
		m.step(start.Add(2*time.Second), city.Calm)

		t.Run("it should keep the ambient clock where it was", func(t *testing.T) {
			assert.InDelta(t, before, m.ambient, 1e-9)
		})
	})
}
