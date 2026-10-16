package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// Item 88: holding the explain key captions what is on screen, and ?
// opens the reference.
func TestExplainAndReference(t *testing.T) {
	home := stageDemo(t, "hamlet")

	t.Run("when e is held", func(t *testing.T) {
		d := newDrive(t, home)
		require.NoError(t, d.Press("hold+e"))
		require.NoError(t, d.Run(2))

		t.Run("it should be explaining", func(t *testing.T) {
			assert.True(t, d.Shown().Explaining)
		})

		t.Run("it should caption the sessions on screen", func(t *testing.T) {
			assert.Contains(t, d.Shown().Captions, "session")
		})

		t.Run("and then released", func(t *testing.T) {
			require.NoError(t, d.Press("release+e"))
			require.NoError(t, d.Run(1))

			t.Run("it should stop explaining", func(t *testing.T) {
				assert.False(t, d.Shown().Explaining)
			})
		})
	})

	t.Run("when ? is pressed", func(t *testing.T) {
		d := newDrive(t, home)
		require.NoError(t, d.Press("?"))
		_, err := d.Until(func(s render.Shown) bool { return s.Help }, settleTicks)

		t.Run("it should open the reference", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})
}
