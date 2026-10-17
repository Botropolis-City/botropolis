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

// Item 89: at fit, and whenever a building would draw too small to read,
// a district draws as a badge carrying its sign.
func TestSemanticZoom(t *testing.T) {
	cityOf := func(districts int) []state.Session {
		var out []state.Session
		for i := 0; i < districts; i++ {
			s := session(fmt.Sprintf("s%02d", i), fmt.Sprintf("/home/demo/src/p%02d", i), state.Working)
			if i == 0 {
				s.State = state.NeedsYou
			}
			out = append(out, s)
		}
		out = append(out, session("extra", "/home/demo/src/p00", state.Working))
		return out
	}
	scene := func(t *testing.T, width, height float64, sessions []state.Session) *city.Scene {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(width, height)
		s.SetSnapshot(snapshot(sessions...))
		s.Fit()
		return s
	}

	for _, n := range []int{2, 12} {
		t.Run(fmt.Sprintf("for a city of %d districts", n), func(t *testing.T) {
			t.Run("when the camera is at fit", func(t *testing.T) {
				s := scene(t, 1600, 1000, cityOf(n))

				t.Run("it should draw badges", func(t *testing.T) {
					assert.True(t, s.Badged())
				})
			})

			t.Run("when the camera is past fit", func(t *testing.T) {
				// A small window keeps fit far out, so the drawn-size
				// threshold decides on its own.
				s := scene(t, 400, 280, cityOf(n))
				threshold := city.BadgeZoom(s.Camera().Projection)
				require.Greater(t, threshold*0.9, s.FitZoom())

				t.Run("and a building cell draws narrower than BadgeBelow", func(t *testing.T) {
					s.Camera().Zoom = threshold * 0.9

					t.Run("it should draw badges", func(t *testing.T) {
						assert.True(t, s.Badged())
					})
				})

				t.Run("and a building cell draws wider", func(t *testing.T) {
					s.Camera().Zoom = threshold * 1.1

					t.Run("it should draw buildings", func(t *testing.T) {
						assert.False(t, s.Badged())
					})
				})
			})
		})
	}

	t.Run("past fit, the threshold should not depend on the number of districts", func(t *testing.T) {
		small, large := scene(t, 1600, 1000, cityOf(2)), scene(t, 1600, 1000, cityOf(12))
		assert.Equal(t, city.BadgeZoom(small.Camera().Projection), city.BadgeZoom(large.Camera().Projection))
	})

	t.Run("when badges are drawn", func(t *testing.T) {
		s := scene(t, 1600, 1000, cityOf(2))
		badges := s.Badges()
		require.Len(t, badges, 2)
		var first city.Badge
		for _, b := range badges {
			if b.District.Root == "/home/demo/src/p00" {
				first = b
			}
		}
		require.NotNil(t, first.District)

		t.Run("each should carry its district's sign", func(t *testing.T) {
			sign := first.District.Sign()
			assert.Equal(t, [2]string{sign.Name, sign.Move}, [2]string{first.Sign.Name, first.Sign.Move})
		})

		t.Run("it should count its sessions and how many need you", func(t *testing.T) {
			assert.Equal(t, [2]int{2, 1}, [2]int{first.Sessions, first.NeedsYou})
		})

		t.Run("and one is selected", func(t *testing.T) {
			s.Click(s.Camera().WorldToScreen(first.District.Rect.Center()))

			t.Run("it should fly to its district, at a zoom where buildings draw", func(t *testing.T) {
				assert.False(t, s.Badged())
			})

			t.Run("it should centre on it", func(t *testing.T) {
				size := s.Size()
				centre := s.Camera().WorldToScreen(first.District.Rect.Center())
				assert.InDelta(t, size.X/2, centre.X, size.X/4)
			})
		})
	})

	t.Run("when the pointer is over a district drawn as a badge", func(t *testing.T) {
		s := scene(t, 1600, 1000, cityOf(2))
		var d *city.District
		for _, b := range s.Badges() {
			d = b.District
		}
		require.NotNil(t, d)
		s.PointerMove(s.Camera().WorldToScreen(d.Rect.Center()))

		t.Run("the badge should take the hit", func(t *testing.T) {
			assert.Equal(t, [2]any{d, (*city.Building)(nil)}, [2]any{s.Hover().Badge, s.Hover().Building})
		})
	})
}
