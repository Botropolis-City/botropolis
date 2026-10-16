package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 88: w walks what waits on you, and esc leaves the walk.
func TestTriageWalk(t *testing.T) {
	home := stageDemo(t, "hamlet")

	t.Run("when w is pressed", func(t *testing.T) {
		d := newDrive(t, home)
		require.NoError(t, d.Press("w"))
		require.NoError(t, d.Run(1))

		t.Run("it should walk, starting on a session", func(t *testing.T) {
			assert.NotEmpty(t, d.Shown().Triage)
		})

		t.Run("it should select the session it is on", func(t *testing.T) {
			assert.Equal(t, d.Shown().Triage, d.Shown().Selected)
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			require.NoError(t, d.Press("Escape"))
			require.NoError(t, d.Run(1))

			t.Run("it should leave the walk", func(t *testing.T) {
				assert.Empty(t, d.Shown().Triage)
			})
		})
	})
}
