package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: the isometric map zooms out no further than where a building
// cell still draws FloorBelow wide, and Skyline is the whole city if it
// reads there, otherwise that floor centred on the city.
func TestSkyline(t *testing.T) {
	cityOf := func(districts int) []state.Session {
		var out []state.Session
		for i := 0; i < districts; i++ {
			out = append(out, session(fmt.Sprintf("s%02d", i), fmt.Sprintf("/home/demo/src/p%02d", i), state.Working))
		}
		return out
	}
	open := func(t *testing.T, width, height float64, sessions []state.Session) *city.Scene {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetProjection(city.Isometric)
		s.Resize(width, height)
		s.SetSnapshot(snapshot(sessions...))
		return s
	}
	settle := func(s *city.Scene) {
		for i := 0; i < 200; i++ {
			s.Animate(1)
		}
	}

	t.Run("no zoom step should go below the floor", func(t *testing.T) {
		var below []float64
		for _, step := range city.ZoomSteps {
			if step < city.FloorZoom(city.Isometric) {
				below = append(below, step)
			}
		}
		assert.Empty(t, below)
	})

	for _, c := range []struct {
		name          string
		districts     int
		width, height float64
		reads         bool
	}{
		{"a city that reads whole at the floor", 2, 1600, 1000, true},
		{"a city that overflows at the floor", 12, 400, 280, false},
	} {
		t.Run("for "+c.name, func(t *testing.T) {
			floor := city.FloorZoom(city.Isometric)

			t.Run("when the wheel is turned out ten notches", func(t *testing.T) {
				s := open(t, c.width, c.height, cityOf(c.districts))
				s.Wheel(s.Size().Scale(0.5), -10)
				settle(s)

				t.Run("it should stop at the floor, not below", func(t *testing.T) {
					assert.GreaterOrEqual(t, s.Camera().Zoom, floor-1e-9)
				})
			})

			t.Run("when Skyline is asked for", func(t *testing.T) {
				s := open(t, c.width, c.height, cityOf(c.districts))
				require.Equal(t, c.reads, s.FitZoom() >= floor, "the case's precondition")
				s.Skyline()

				if c.reads {
					t.Run("it should equal the fit", func(t *testing.T) {
						assert.InDelta(t, s.FitZoom(), s.Camera().Zoom, 1e-9)
					})
				} else {
					t.Run("it should be the floor", func(t *testing.T) {
						assert.InDelta(t, floor, s.Camera().Zoom, 1e-9)
					})

					t.Run("it should centre on the city", func(t *testing.T) {
						centre := s.Camera().WorldToScreen(s.City().Extent().Center())
						in := s.Insets()
						size := s.Size()
						assert.InDelta(t, in.Left+(size.X-in.Left-in.Right)/2, centre.X, 1)
					})
				}
			})

			t.Run("when the map opens", func(t *testing.T) {
				s := open(t, c.width, c.height, cityOf(c.districts))

				t.Run("it should open at Skyline", func(t *testing.T) {
					assert.InDelta(t, max(s.FitZoom(), floor), s.Camera().Zoom, 1e-9)
				})
			})
		})
	}
}
