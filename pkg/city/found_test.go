package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestFoundFilter(t *testing.T) {
	a := session("a", cinders, state.Working)
	b := session("b", botropolis, state.NeedsYou)
	c := session("c", botropolis, state.Waiting)
	dimmed := func(s *city.Scene) map[string]bool {
		out := map[string]bool{}
		for _, bld := range s.City().Buildings() {
			out[bld.Session.ID] = s.Dimmed(bld)
		}
		return out
	}

	t.Run("when the map is filtered to a set of sessions", func(t *testing.T) {
		s := scene(t, a, b, c)
		s.SetFound("ci queue", []string{"a", "c", "gone-from-the-map"})

		t.Run("it should dim every building outside it", func(t *testing.T) {
			assert.Equal(t, map[string]bool{"a": false, "b": true, "c": false}, dimmed(s))
		})

		t.Run("it should count the ones inside it as matching", func(t *testing.T) {
			assert.Equal(t, 2, s.Matching())
		})
	})

	t.Run("when the set is cleared", func(t *testing.T) {
		s := scene(t, a, b, c)
		s.SetFound("ci queue", []string{"a"})
		s.SetFilter("")

		t.Run("it should dim nothing", func(t *testing.T) {
			assert.Equal(t, map[string]bool{"a": false, "b": false, "c": false}, dimmed(s))
		})
	})
}
