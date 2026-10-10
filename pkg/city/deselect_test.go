package city_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bug 43a. Aria, on a frame of an open building card: "escape should
// close menus like these."

func TestDeselect(t *testing.T) {
	t.Run("when a building is selected and its card is open", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		require.True(t, s.Select("a"))
		require.NotNil(t, s.Selected())
		s.Deselect()

		t.Run("it should close the selection", func(t *testing.T) {
			assert.Nil(t, s.Selected())
		})
	})

	t.Run("when nothing is selected", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Deselect()

		t.Run("it should stay closed rather than complain", func(t *testing.T) {
			assert.Nil(t, s.Selected())
		})
	})
}
