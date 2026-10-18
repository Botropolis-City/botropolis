package city_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 112: the City Plan drawn as a map: an inked coast round the
// city's land, and the plaza as a glyph no badge covers.
func TestCityPlanCharacter(t *testing.T) {
	open := func(t *testing.T, districts, heading int, width, height float64) *city.Scene {
		t.Helper()
		var sessions []state.Session
		for i := 0; i < districts; i++ {
			sessions = append(sessions, session(fmt.Sprintf("s%03d", i), fmt.Sprintf("/home/demo/src/workshop-service-%03d", i), state.Working))
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetProjection(city.Isometric)
		s.SetInstant(true)
		s.Resize(width, height)
		s.SetSnapshot(snapshot(sessions...))
		s.Camera().Heading = heading
		s.ToggleCityPlan()
		return s
	}
	inside := func(poly []city.Point, p city.Point) bool {
		in := false
		for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
			a, b := poly[i], poly[j]
			if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
				in = !in
			}
		}
		return in
	}
	unringed := func(plan city.CityPlan) []string {
		var out []string
		for _, b := range plan.Badges {
			for _, corner := range b.Lot {
				if !inside(plan.Coast, corner) {
					out = append(out, b.Name)
					break
				}
			}
		}
		return out
	}
	// margins is how far each point of the coast lies from the nearest
	// land, a district's lot or the plaza, in cells.
	margins := func(s *city.Scene, plan city.CityPlan) []float64 {
		land := []city.Rect{s.City().Plaza}
		for _, d := range s.City().Districts {
			if !d.Depot {
				land = append(land, d.Rect)
			}
		}
		var out []float64
		for _, p := range plan.Coast {
			w := plan.ToWorld(p)
			nearest := math.Inf(1)
			for _, r := range land {
				dx := math.Max(0, math.Max(r.Min.X-w.X, w.X-r.Max.X))
				dy := math.Max(0, math.Max(r.Min.Y-w.Y, w.Y-r.Max.Y))
				nearest = math.Min(nearest, math.Hypot(dx, dy))
			}
			out = append(out, nearest/city.CellSize)
		}
		return out
	}

	t.Run("when the plan is open on 6 districts", func(t *testing.T) {
		s := open(t, 6, 0, 1600, 1000)
		plan := s.CityPlan()
		require.NotEmpty(t, plan.Coast, "precondition: the plan has a coast")

		t.Run("the coast", func(t *testing.T) {
			t.Run("it should ring every district's lot", func(t *testing.T) {
				assert.Empty(t, unringed(plan))
			})

			t.Run("it should lie inside the plan's area", func(t *testing.T) {
				var out []city.Point
				for _, p := range plan.Coast {
					if !plan.Area.Inset(-0.5).Contains(p) {
						out = append(out, p)
					}
				}
				assert.Empty(t, out)
			})

			t.Run("it should keep about one cell's margin round the land", func(t *testing.T) {
				var off []float64
				for _, m := range margins(s, plan) {
					if m < 0.5 || m > 1.5 {
						off = append(off, m)
					}
				}
				assert.Empty(t, off)
			})

			t.Run("it should wander", func(t *testing.T) {
				ms := margins(s, plan)
				lo, hi := ms[0], ms[0]
				for _, m := range ms {
					lo, hi = math.Min(lo, m), math.Max(hi, m)
				}
				assert.GreaterOrEqual(t, hi-lo, 0.25)
			})

			t.Run("and the map is turned", func(t *testing.T) {
				turned := open(t, 6, 90, 1600, 1000).CityPlan()

				t.Run("it should still ring every district's lot", func(t *testing.T) {
					assert.Empty(t, unringed(turned))
				})
			})
		})

		t.Run("the plaza", func(t *testing.T) {
			require.Positive(t, plan.Plaza.Area(), "precondition: the plan has a plaza glyph")

			t.Run("it should put the glyph at the plaza's place on the plan", func(t *testing.T) {
				want := plan.ToScreen(s.City().Plaza.Center())
				got := plan.Plaza.Center()
				assert.InDelta(t, 0, math.Hypot(got.X-want.X, got.Y-want.Y), 1)
			})
		})
	})

	// Each of these puts a badge on the plaza's place unless the glyph
	// keeps it clear.
	for _, c := range []struct {
		districts     int
		width, height float64
	}{
		{6, 1600, 1000},
		{6, 600, 400},
		{40, 900, 600},
	} {
		t.Run(fmt.Sprintf("when the plan holds %d districts at %.0fx%.0f", c.districts, c.width, c.height), func(t *testing.T) {
			plan := open(t, c.districts, 0, c.width, c.height).CityPlan()
			require.Positive(t, plan.Plaza.Area(), "precondition: the plan has a plaza glyph")

			t.Run("the plaza glyph should overlap no badge", func(t *testing.T) {
				var over []string
				for _, b := range plan.Badges {
					if b.Rect.Overlaps(plan.Plaza) {
						over = append(over, b.Name)
					}
				}
				assert.Empty(t, over)
			})
		})
	}
}
