package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

// Item 185: the power plant's windows are lit while any session is
// working, coming on and going off over about a second, as a building's
// do, so a session flicking between working and waiting does not strobe
// it.
func TestPlantLights(t *testing.T) {
	t.Run("when the city opens with a session working", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))

		t.Run("it should light the plant at once", func(t *testing.T) {
			assert.Equal(t, 1.0, s.PlantLights())
		})
	})

	t.Run("when the city opens with nobody working", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Waiting))

		t.Run("it should leave the plant dark", func(t *testing.T) {
			assert.Equal(t, 0.0, s.PlantLights())
		})
	})

	for _, c := range []struct {
		name     string
		from, to state.State
		after    float64
		want     float64
	}{
		{"a session begins working, and half a second passes", state.Waiting, state.Working, 0.5, 0.5},
		{"a session begins working, and a second passes", state.Waiting, state.Working, 1, 1},
		{"the last working session stops, and half a second passes", state.Working, state.Waiting, 0.5, 0.5},
		{"the last working session stops, and a second passes", state.Working, state.Waiting, 1, 0},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			s := scene(t, session("a", cinders, c.from))
			s.Animate(0.1)
			s.SetSnapshot(snapshot(session("a", cinders, c.to)))
			for range int(c.after * 10) {
				s.Animate(0.1)
			}

			t.Run("it should have the plant's lights that far on", func(t *testing.T) {
				assert.InDelta(t, c.want, s.PlantLights(), 1e-9)
			})
		})
	}

	t.Run("when motion is reduced and a session begins working", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Waiting))
		s.SetReducedMotion(true)
		s.Animate(0.1)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
		s.Animate(0.1)

		t.Run("it should light the plant at once", func(t *testing.T) {
			assert.Equal(t, 1.0, s.PlantLights())
		})
	})
}
