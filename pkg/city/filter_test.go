package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestFilter(t *testing.T) {
	a := session("a", cinders, state.Working)
	a.Title, a.Branch, a.Model = "Fix the CI queue", "main", "claude-opus-5"
	b := session("b", botropolis, state.NeedsYou)
	b.Title, b.Branch, b.Model = "Phase 7 UI", "feat/ui", "claude-sonnet-5"
	c := build(t, city.NewLayout(), a, b)
	building := func(id string) *city.Building {
		for _, bld := range c.Buildings() {
			if bld.Session.ID == id {
				return bld
			}
		}
		return nil
	}

	cases := []struct {
		query string
		a, b  bool
	}{
		{"", true, true},
		{"queue", true, false},
		{"QUEUE", true, false},
		{"botropolis", false, true},
		{"feat/", false, true},
		{"needs-you", false, true},
		{"sonnet", false, true},
		{"claude main", true, false},
		{"zzz", false, false},
	}
	for _, tc := range cases {
		f := city.ParseFilter(tc.query)
		t.Run("when the filter is "+tc.query, func(t *testing.T) {
			t.Run("it should decide for a", func(t *testing.T) {
				assert.Equal(t, tc.a, f.Matches(building("a")))
			})

			t.Run("it should decide for b", func(t *testing.T) {
				assert.Equal(t, tc.b, f.Matches(building("b")))
			})
		})
	}

	t.Run("when the filter is empty", func(t *testing.T) {
		t.Run("it should be nothing", func(t *testing.T) {
			assert.True(t, city.ParseFilter("  ").Empty())
		})
	})

	t.Run("when the scene is filtered", func(t *testing.T) {
		s := scene(t, a, b)
		s.SetFilter("queue")

		t.Run("it should dim what does not match", func(t *testing.T) {
			assert.False(t, s.Dimmed(building("a")))
			assert.True(t, s.Dimmed(s.City().Buildings()[0]) != s.Dimmed(s.City().Buildings()[1]))
		})

		t.Run("it should count the matches", func(t *testing.T) {
			assert.Equal(t, 1, s.Matching())
		})

		t.Run("and the filter is cleared", func(t *testing.T) {
			s.SetFilter("")

			t.Run("it should dim nothing", func(t *testing.T) {
				for _, bld := range s.City().Buildings() {
					assert.False(t, s.Dimmed(bld))
				}
			})
		})
	})
}
