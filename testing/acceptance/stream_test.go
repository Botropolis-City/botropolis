package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 88: the stream is a column down the right, shown until l hides it.
func TestStreamColumn(t *testing.T) {
	home := stageDemo(t, "hamlet")

	t.Run("when the city opens", func(t *testing.T) {
		d := newDrive(t, home)
		require.NoError(t, d.Run(1))

		t.Run("it should show the stream", func(t *testing.T) {
			assert.True(t, d.Shown().Stream)
		})

		t.Run("and l is pressed", func(t *testing.T) {
			require.NoError(t, d.Press("l"))
			require.NoError(t, d.Run(1))

			t.Run("it should hide the column", func(t *testing.T) {
				assert.False(t, d.Shown().Stream)
			})

			t.Run("and pressed again", func(t *testing.T) {
				require.NoError(t, d.Press("l"))
				require.NoError(t, d.Run(1))

				t.Run("it should show the column", func(t *testing.T) {
					assert.True(t, d.Shown().Stream)
				})
			})
		})
	})
}
