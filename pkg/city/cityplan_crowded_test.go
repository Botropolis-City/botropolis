package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 106: a few dozen directories fit; badges shrink as the plan
// fills, and none overlap.
func TestCityPlanCrowded(t *testing.T) {
	for _, c := range []struct {
		districts int
		compact   bool
	}{
		{10, false},
		{40, true},
		{100, true},
	} {
		t.Run(fmt.Sprintf("when the city has %d districts", c.districts), func(t *testing.T) {
			var sessions []state.Session
			for i := 0; i < c.districts; i++ {
				sessions = append(sessions, session(fmt.Sprintf("s%03d", i), fmt.Sprintf("/home/demo/src/workshop-service-%03d", i), state.Working))
			}
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return now }, time.UTC)
			s.SetProjection(city.Isometric)
			s.SetInstant(true)
			s.Resize(1600, 1000)
			s.SetSnapshot(snapshot(sessions...))
			plan := s.CityPlan()

			t.Run("it should give every district one badge", func(t *testing.T) {
				assert.Len(t, plan.Badges, c.districts)
			})

			t.Run("it should overlap no two badges", func(t *testing.T) {
				var overlaps []string
				for i, a := range plan.Badges {
					grown := a.Rect.Inset(-2)
					for _, b := range plan.Badges[i+1:] {
						if grown.Overlaps(b.Rect) {
							overlaps = append(overlaps, a.Name+" / "+b.Name)
						}
					}
				}
				assert.Empty(t, overlaps)
			})

			t.Run("it should keep every badge inside the plan's area", func(t *testing.T) {
				var outside []string
				for _, b := range plan.Badges {
					if b.Rect.Min.X < plan.Area.Min.X-1e-6 || b.Rect.Min.Y < plan.Area.Min.Y-1e-6 || b.Rect.Max.X > plan.Area.Max.X+1e-6 || b.Rect.Max.Y > plan.Area.Max.Y+1e-6 {
						outside = append(outside, b.Name)
					}
				}
				assert.Empty(t, outside)
			})

			t.Run("it should keep the left-to-right order of districts more than a badge apart on the map", func(t *testing.T) {
				var swapped []string
				for _, a := range plan.Badges {
					for _, b := range plan.Badges {
						if b.Place.X-a.Place.X > a.Rect.Width()+b.Rect.Width() && a.At.X >= b.At.X {
							swapped = append(swapped, a.Name+" / "+b.Name)
						}
					}
				}
				assert.Empty(t, swapped)
			})

			want := "full"
			if c.compact {
				want = "compact"
			}
			t.Run("its badges should be "+want, func(t *testing.T) {
				var wrong []string
				for _, b := range plan.Badges {
					if b.Compact != c.compact {
						wrong = append(wrong, b.Name)
					}
				}
				assert.Empty(t, wrong)
			})
		})
	}
}

// Item 106: the plan's badges are pushed apart, inside its area.
func TestPlanLayout(t *testing.T) {
	area := city.RectAt(0, 0, 400, 300)
	box := func(x, y float64) city.Rect { return city.RectAt(x, y, 100, 40) }

	t.Run("when badges are already apart", func(t *testing.T) {
		in := []city.Rect{box(10, 10), box(200, 200)}
		out := city.LayoutPlanBadges(in, area, 4)

		t.Run("none should move", func(t *testing.T) {
			assert.Equal(t, in, out)
		})
	})

	t.Run("when two badges stand on the same spot", func(t *testing.T) {
		out := city.LayoutPlanBadges([]city.Rect{box(150, 130), box(150, 130)}, area, 4)

		t.Run("they should end at least the gap apart", func(t *testing.T) {
			grown := out[0].Inset(-3.9)
			assert.False(t, grown.Overlaps(out[1]))
		})

		t.Run("they should part on the axis of least overlap, one above the other", func(t *testing.T) {
			assert.InDelta(t, out[0].Min.X, out[1].Min.X, 1e-9)
		})
	})

	t.Run("when a badge is pressed against the area's edge", func(t *testing.T) {
		out := city.LayoutPlanBadges([]city.Rect{box(0, 0), box(10, 5)}, area, 4)

		t.Run("it should stay inside", func(t *testing.T) {
			var outside []int
			for i, r := range out {
				if r.Min.X < area.Min.X || r.Min.Y < area.Min.Y || r.Max.X > area.Max.X || r.Max.Y > area.Max.Y {
					outside = append(outside, i)
				}
			}
			assert.Empty(t, outside)
		})
	})
}
