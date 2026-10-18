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

// Item 112: the City Plan carries a compass that turns with the map and
// a cartouche with its title and the time, each in a top corner, clear
// of the badges and of whatever chrome covers the plan.
func TestCityPlanCompassAndCartouche(t *testing.T) {
	open := func(t *testing.T, districts int, width, height float64, covers []city.Rect) *city.Scene {
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
		s.SetPlanCovers(covers)
		s.ToggleCityPlan()
		return s
	}
	north := func(plan city.CityPlan, s *city.Scene) city.Point {
		c := s.City().Plaza.Center()
		d := plan.ToScreen(c.Add(city.Point{Y: -city.CellSize})).Sub(plan.ToScreen(c))
		return d.Scale(1 / math.Hypot(d.X, d.Y))
	}
	off := func(a, b city.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

	t.Run("when the plan is open on 6 districts", func(t *testing.T) {
		s := open(t, 6, 1600, 1000, nil)
		plan := s.CityPlan()
		area := plan.Area
		require.Positive(t, plan.Compass.Box.Area(), "precondition: the plan has a compass")
		require.Positive(t, plan.Cartouche.Box.Area(), "precondition: the plan has a cartouche")

		t.Run("the compass", func(t *testing.T) {
			t.Run("it should stand in the top-right corner of the plan's area", func(t *testing.T) {
				c := plan.Compass.Box.Center()
				assert.True(t, c.X > area.Max.X-area.Width()/4 && c.Y < area.Min.Y+area.Height()/4, "at %v in %v", c, area)
			})

			t.Run("it should point north as the plan draws it", func(t *testing.T) {
				assert.InDelta(t, 0, off(plan.Compass.North, north(plan, s)), 1e-6)
			})

			t.Run("and r is pressed", func(t *testing.T) {
				turned := open(t, 6, 1600, 1000, nil)
				turned.Turn()
				after := turned.CityPlan()

				t.Run("it should turn the needle by a quarter turn", func(t *testing.T) {
					a, b := plan.Compass.North, after.Compass.North
					assert.InDelta(t, 0, a.X*b.X+a.Y*b.Y, 1e-6)
				})

				t.Run("it should still point north as the plan draws it", func(t *testing.T) {
					assert.InDelta(t, 0, off(after.Compass.North, north(after, turned)), 1e-6)
				})
			})
		})

		t.Run("the cartouche", func(t *testing.T) {
			t.Run("it should stand in the top-left corner of the plan's area", func(t *testing.T) {
				c := plan.Cartouche.Box.Center()
				assert.True(t, c.X < area.Min.X+area.Width()/4 && c.Y < area.Min.Y+area.Height()/4, "at %v in %v", c, area)
			})

			t.Run("it should read City Plan", func(t *testing.T) {
				assert.Equal(t, "City Plan", plan.Cartouche.Title)
			})

			t.Run("it should carry the city clock's time", func(t *testing.T) {
				assert.Equal(t, "08:00", plan.Cartouche.Time)
			})

			t.Run("and the clock is scrubbed an hour on", func(t *testing.T) {
				scrubbed := open(t, 6, 1600, 1000, nil)
				scrubbed.Scrub(time.Hour)

				t.Run("it should carry the scrubbed time", func(t *testing.T) {
					assert.Equal(t, "09:00", scrubbed.CityPlan().Cartouche.Time)
				})
			})
		})
	})

	t.Run("when the stream's corner is reserved along the top", func(t *testing.T) {
		s := open(t, 40, 1920, 1080, nil)
		corner := city.RectAt(1600, 32, 320, 190)
		s.SetCornerChrome(corner)
		require.Greater(t, s.Insets().Top, corner.Max.Y, "precondition: the map reserves the corner along the top")
		plan := s.CityPlan()

		t.Run("the plan's area should keep the band beside the corner", func(t *testing.T) {
			assert.Less(t, plan.Area.Min.Y, corner.Max.Y)
		})

		t.Run("no badge should overlap the corner", func(t *testing.T) {
			var over []string
			for _, b := range plan.Badges {
				if b.Rect.Overlaps(corner) {
					over = append(over, b.Name)
				}
			}
			assert.Empty(t, over)
		})

		t.Run("the compass should overlap the corner nowhere", func(t *testing.T) {
			assert.False(t, plan.Compass.Box.Overlaps(corner))
		})
	})

	// A cover a third of the window across and down, in the top-left
	// corner where the lens panel sits.
	topLeft := func(width, height float64) []city.Rect {
		return []city.Rect{city.RectAt(0, 40, width/3, height/3)}
	}
	for _, c := range []struct {
		districts     int
		width, height float64
		covered       bool
	}{
		{6, 1600, 1000, false},
		{6, 600, 400, false},
		{40, 900, 600, false},
		{20, 600, 400, false},
		{100, 900, 600, false},
		{6, 600, 400, true},
		{40, 900, 600, true},
		{100, 1600, 1000, true},
	} {
		var covers []city.Rect
		if c.covered {
			covers = topLeft(c.width, c.height)
		}
		c := struct {
			districts     int
			width, height float64
			covers        []city.Rect
		}{c.districts, c.width, c.height, covers}
		t.Run(fmt.Sprintf("when the plan holds %d districts at %.0fx%.0f under %d covers", c.districts, c.width, c.height, len(c.covers)), func(t *testing.T) {
			plan := open(t, c.districts, c.width, c.height, c.covers).CityPlan()
			marks := map[string]city.Rect{"compass": plan.Compass.Box, "cartouche": plan.Cartouche.Box}
			require.Positive(t, plan.Compass.Box.Area(), "precondition: the plan has a compass")
			require.Positive(t, plan.Cartouche.Box.Area(), "precondition: the plan has a cartouche")

			for name, box := range marks {
				t.Run("the "+name+" should overlap no badge", func(t *testing.T) {
					var over []string
					for _, b := range plan.Badges {
						if b.Rect.Overlaps(box) {
							over = append(over, b.Name)
						}
					}
					assert.Empty(t, over)
				})

				t.Run("the "+name+" should overlap no cover", func(t *testing.T) {
					var over []city.Rect
					for _, r := range c.covers {
						if r.Overlaps(box) {
							over = append(over, r)
						}
					}
					assert.Empty(t, over)
				})
			}

			t.Run("no badge should overlap a cover", func(t *testing.T) {
				var over []string
				for _, b := range plan.Badges {
					for _, r := range c.covers {
						if b.Rect.Overlaps(r) {
							over = append(over, b.Name)
						}
					}
				}
				assert.Empty(t, over)
			})
		})
	}
}
