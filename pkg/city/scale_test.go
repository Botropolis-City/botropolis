package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 270: a frame asks every building for its inspector, so asking
// one must not cost a pass over every building.
const scaleBuildings, scaleDistricts = 600, 200

func scaleSessions(open string) []state.Session {
	sessions := make([]state.Session, 0, scaleBuildings)
	for i := range scaleBuildings {
		id := fmt.Sprintf("scale-%04d", i)
		s := session(id, fmt.Sprintf("/w/scale%03d", i%scaleDistricts), state.Working)
		s.SubagentsInFlight = 0
		if id == open {
			s.PRs = []claude.PR{{Number: 1, URL: "https://example.com/o/r/pull/1", Repository: "o/r", State: claude.PROpen}}
		}
		sessions = append(sessions, s)
	}
	return sessions
}

func TestBuildingsOfABuiltCity(t *testing.T) {
	t.Run("when a city of several districts is built", func(t *testing.T) {
		c := city.Build(snapshot(scaleSessions("")...), city.NewLayout())
		var want []string
		for _, d := range c.Districts {
			for _, b := range d.Buildings {
				want = append(want, b.Session.ID)
			}
		}
		var got []string
		for _, b := range c.Buildings() {
			got = append(got, b.Session.ID)
		}

		t.Run("it should list every district's buildings, in district order", func(t *testing.T) {
			assert.Equal(t, want, got)
		})

		t.Run("and Buildings is asked again", func(t *testing.T) {
			t.Run("it should allocate nothing", func(t *testing.T) {
				assert.Zero(t, testing.AllocsPerRun(10, func() { _ = c.Buildings() }))
			})
		})
	})
}

func TestInspectorAtScale(t *testing.T) {
	open := func(parked string) *city.Scene {
		start := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return start.Add(time.Minute) }, time.UTC)
		s.SetSnapshot(snapshot(scaleSessions(parked)...))
		return s
	}
	every := func(s *city.Scene) (found []string) {
		for _, b := range s.City().Buildings() {
			if _, ok := s.Inspector(b.Session.ID); ok {
				found = append(found, b.Session.ID)
			}
		}
		return found
	}

	t.Run("when a scene of many buildings has nothing parked", func(t *testing.T) {
		s := open("")

		t.Run("and every building's inspector is asked for, as a frame does", func(t *testing.T) {
			t.Run("it should allocate nothing", func(t *testing.T) {
				assert.Zero(t, testing.AllocsPerRun(5, func() { _ = every(s) }))
			})

			t.Run("it should find none", func(t *testing.T) {
				assert.Empty(t, every(s))
			})
		})
	})

	t.Run("when one building of many is parked", func(t *testing.T) {
		s := open("scale-0421")

		t.Run("it should find that building's inspector, and no other building's", func(t *testing.T) {
			assert.Equal(t, []string{"scale-0421"}, every(s))
		})
	})
}
