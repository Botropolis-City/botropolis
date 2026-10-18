package render

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 106: the City Plan is drawn in place of the isometric map, one
// zoom step past Skyline.
func TestCityPlanDraw(t *testing.T) {
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := 0; i < 6; i++ {
		cwd := fmt.Sprintf("/p/d%d", i)
		sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at, SubagentsInFlight: 1})
	}
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return at }, time.UTC)
	s.SetProjection(city.Isometric)
	s.SetInstant(true)
	s.Resize(1600, 1000)
	s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
	s.Skyline()
	s.Wheel(s.Size().Scale(0.5), -1)
	require.True(t, s.CityPlanOpen(), "precondition: the plan is open")
	plan := s.CityPlan()

	t.Run("when the plan is open", func(t *testing.T) {
		hits := planHits(plan)

		t.Run("it should note a hit per badge, standing for its district", func(t *testing.T) {
			var want, got []*city.District
			for _, b := range plan.Badges {
				want = append(want, b.District)
			}
			for _, h := range hits {
				got = append(got, h.hit.District)
			}
			assert.Equal(t, want, got)
		})

		t.Run("it should draw no buildings, drones or movers", func(t *testing.T) {
			var movers []string
			for _, h := range hits {
				if h.hit.Building != nil || h.hit.Mover() {
					movers = append(movers, fmt.Sprint(h.hit))
				}
			}
			assert.Empty(t, movers)
		})
	})

	t.Run("the help should say how to reach the City Plan", func(t *testing.T) {
		var rows []string
		for _, k := range bindings {
			if strings.Contains(k.Action, "City Plan") {
				rows = append(rows, k.Key)
			}
		}
		assert.NotEmpty(t, rows)
	})
}
