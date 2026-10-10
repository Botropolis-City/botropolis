package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestParseDetail(t *testing.T) {
	for _, name := range []string{"full", "plain"} {
		t.Run("when the setting says "+name, func(t *testing.T) {
			t.Run("it should be taken", func(t *testing.T) {
				d, ok := city.ParseDetail(name)
				require.True(t, ok)
				assert.Equal(t, name, d.String())
			})
		})
	}

	t.Run("when the setting says something else", func(t *testing.T) {
		t.Run("it should refuse rather than guess", func(t *testing.T) {
			_, ok := city.ParseDetail("medium")
			assert.False(t, ok)
		})
	})
}

func TestSceneryDetail(t *testing.T) {
	t.Run("when a scene is new", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Working))

		t.Run("it should be full, because the city is the point", func(t *testing.T) {
			assert.Equal(t, city.DetailFull, s.Detail())
		})

		t.Run("it should draw the scenery", func(t *testing.T) {
			assert.True(t, s.Scenery())
		})
	})

	t.Run("when the detail is turned down", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Working))
		s.SetDetail(city.DetailPlain)

		t.Run("it should stop drawing the scenery", func(t *testing.T) {
			assert.False(t, s.Scenery())
		})

		t.Run("it should keep the city itself, which is not scenery", func(t *testing.T) {
			assert.NotEmpty(t, s.City().Buildings())
		})
	})

	t.Run("when the detail changes", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Working))
		before := s.Generation()
		s.SetDetail(city.DetailPlain)

		t.Run("it should count as a new city, so the composed layer is drawn again", func(t *testing.T) {
			assert.NotEqual(t, before, s.Generation())
		})
	})

	t.Run("when the detail is set to what it already is", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Working))
		before := s.Generation()
		s.SetDetail(city.DetailFull)

		t.Run("it should leave the layer alone", func(t *testing.T) {
			assert.Equal(t, before, s.Generation())
		})
	})
}
