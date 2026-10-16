package acceptance_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 88: the number keys pick the six lenses, and a lens's key pressed
// again steps to its next view.
func TestLensKeys(t *testing.T) {
	home := stageDemo(t, "hamlet")
	press := func(t *testing.T, keys ...string) [2]string {
		t.Helper()
		d := newDrive(t, home)
		for _, k := range keys {
			require.NoError(t, d.Press(k))
		}
		require.NoError(t, d.Run(1))
		s := d.Shown()
		return [2]string{s.Lens, s.View}
	}
	lenses := []string{"Attention", "Work", "Cost", "Capacity", "Health", "Connections"}

	for i, lens := range lenses {
		key := fmt.Sprint("digit", i+1)
		t.Run("when "+key+" is pressed, it should open "+lens, func(t *testing.T) {
			assert.Equal(t, lens, press(t, key)[0])
		})
	}

	t.Run("when 4 is pressed twice, it should step Capacity to its next view", func(t *testing.T) {
		assert.Equal(t, [2]string{"Capacity", "fan-out"}, press(t, "digit4", "digit4"))
	})

	t.Run("when 7 is pressed, it should leave the lens as it was", func(t *testing.T) {
		assert.Equal(t, press(t, "digit3"), press(t, "digit3", "digit7"))
	})
}
