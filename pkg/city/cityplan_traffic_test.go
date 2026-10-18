package city_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 112: the traffic between districts is drawn on the City Plan as
// dashed ink along each road's own streets.
func TestCityPlanTraffic(t *testing.T) {
	open := func(t *testing.T, roads []state.Road, heading int) *city.Scene {
		t.Helper()
		snap := snapshot(threeDistricts()...)
		snap.Roads = roads
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetProjection(city.Isometric)
		s.SetInstant(true)
		s.Resize(1600, 1000)
		s.SetSnapshot(snap)
		s.Camera().Heading = heading
		s.ToggleCityPlan()
		return s
	}
	// strays is every traffic point that is not its street's point on
	// the plan.
	strays := func(s *city.Scene) []city.Point {
		plan := s.CityPlan()
		streets := s.City().Streets
		var out []city.Point
		if len(plan.Traffic) != len(streets) {
			return []city.Point{{X: math.NaN()}}
		}
		for i, tr := range plan.Traffic {
			if len(tr.Points) != len(streets[i].Path) {
				out = append(out, city.Point{X: math.NaN()})
				continue
			}
			for j, p := range tr.Points {
				want := plan.ToScreen(streets[i].Path[j])
				if math.Hypot(p.X-want.X, p.Y-want.Y) > 0.5 {
					out = append(out, p)
				}
			}
		}
		return out
	}
	road := func(active int) []state.Road {
		return []state.Road{{From: "/p/a", To: "/p/c", Files: 2, Sessions: 1, Active: active}}
	}

	t.Run("when two districts exchange files on a road", func(t *testing.T) {
		s := open(t, road(0), 0)
		require.Len(t, s.City().Streets, 1, "precondition: the map draws the road")
		plan := s.CityPlan()

		t.Run("it should draw one traffic line per road", func(t *testing.T) {
			assert.Len(t, plan.Traffic, 1)
		})

		t.Run("it should follow the road's street path on the plan", func(t *testing.T) {
			assert.Empty(t, strays(s))
		})

		t.Run("and the road has a working traveller", func(t *testing.T) {
			busy := open(t, road(1), 0).CityPlan()
			require.Len(t, busy.Traffic, 1)

			t.Run("it should be active", func(t *testing.T) {
				assert.True(t, busy.Traffic[0].Active)
			})
		})

		t.Run("and the road is quiet", func(t *testing.T) {
			require.Len(t, plan.Traffic, 1)

			t.Run("it should not be active", func(t *testing.T) {
				assert.False(t, plan.Traffic[0].Active)
			})
		})

		t.Run("and the map is turned", func(t *testing.T) {
			turned := open(t, road(0), 90)
			require.Len(t, turned.City().Streets, 1)

			t.Run("it should still follow the road's street path", func(t *testing.T) {
				assert.Empty(t, strays(turned))
			})
		})
	})

	t.Run("when no districts exchange anything", func(t *testing.T) {
		plan := open(t, nil, 0).CityPlan()

		t.Run("it should draw no traffic", func(t *testing.T) {
			assert.Empty(t, plan.Traffic)
		})
	})
}
