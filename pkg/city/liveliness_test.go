package city

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 89: how much the city moves is a setting, from still to lively.
func TestLiveliness(t *testing.T) {
	levels := []Liveliness{Still, Calm, Steady, Lively}

	for _, level := range levels {
		t.Run("when parsing "+level.String(), func(t *testing.T) {
			got, ok := ParseLiveliness(level.String())

			t.Run("it should give back that level", func(t *testing.T) {
				require.True(t, ok)
				assert.Equal(t, level, got)
			})
		})
	}

	t.Run("when parsing a name that is not a level", func(t *testing.T) {
		_, ok := ParseLiveliness("frantic")

		t.Run("it should refuse it", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when nothing is set", func(t *testing.T) {
		var level Liveliness

		t.Run("it should be steady", func(t *testing.T) {
			assert.Equal(t, Steady, level)
		})
	})

	t.Run("when the level is still", func(t *testing.T) {
		t.Run("it should run ambient motion at no pace", func(t *testing.T) {
			assert.Zero(t, Still.Pace())
		})

		t.Run("it should cross-fade transitions", func(t *testing.T) {
			assert.True(t, Still.CrossFades())
		})
	})

	for _, level := range levels[1:] {
		t.Run("when the level is "+level.String(), func(t *testing.T) {
			t.Run("it should not cross-fade transitions", func(t *testing.T) {
				assert.False(t, level.CrossFades())
			})
		})
	}

	t.Run("when the levels are taken in order", func(t *testing.T) {
		for i := 1; i < len(levels); i++ {
			t.Run("it should run "+levels[i].String()+" faster than "+levels[i-1].String(), func(t *testing.T) {
				assert.Greater(t, levels[i].Pace(), levels[i-1].Pace())
			})
		}
	})
}
