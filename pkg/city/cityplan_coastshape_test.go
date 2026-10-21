package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 191: the painter keeps the coast's triangulation for as long as
// the coast keeps its shape, so the plan says when the shape changed.
func TestCityPlanCoastShape(t *testing.T) {
	scene := func(districts int) *city.Scene {
		var sessions []state.Session
		for i := 0; i < districts; i++ {
			sessions = append(sessions, session(fmt.Sprintf("s%03d", i), fmt.Sprintf("/home/demo/src/coast-%03d", i), state.Working))
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetProjection(city.Isometric)
		s.SetInstant(true)
		s.Resize(1600, 1000)
		s.SetSnapshot(snapshot(sessions...))
		return s
	}

	t.Run("when the plan is drawn twice over the same city", func(t *testing.T) {
		s := scene(4)
		first := s.CityPlan().CoastShape
		s.Resize(1200, 800)

		t.Run("it should keep the coast's shape, even at another size", func(t *testing.T) {
			assert.Equal(t, first, s.CityPlan().CoastShape)
		})
	})

	t.Run("when a district is added between two plans", func(t *testing.T) {
		s := scene(4)
		first := s.CityPlan().CoastShape
		var sessions []state.Session
		for i := 0; i < 5; i++ {
			sessions = append(sessions, session(fmt.Sprintf("s%03d", i), fmt.Sprintf("/home/demo/src/coast-%03d", i), state.Working))
		}
		s.SetSnapshot(snapshot(sessions...))

		t.Run("it should give the coast a new shape", func(t *testing.T) {
			assert.NotEqual(t, first, s.CityPlan().CoastShape)
		})
	})

	t.Run("when two scenes each draw a plan", func(t *testing.T) {
		t.Run("it should give their coasts different shapes", func(t *testing.T) {
			assert.NotEqual(t, scene(4).CityPlan().CoastShape, scene(4).CityPlan().CoastShape)
		})
	})
}
