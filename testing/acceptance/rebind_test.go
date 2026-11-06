package acceptance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350, slice 5b: a key is rebound from the settings' Keys panel as a
// person would, takes effect at once, and is written to the config file.
func TestRebindFromSettings(t *testing.T) {
	home := stageDemo(t, "hamlet")
	file := filepath.Join(t.TempDir(), "config.toml")
	d, err := render.NewDrive(render.DriveOptions{
		Home:     home,
		Settings: ui.Settings{Items: []ui.Setting{render.KeysRow}},
		Apply: func(s ui.Setting) error {
			name, _ := strings.CutPrefix(s.Key, config.KeySettings+".")
			return config.SaveBinding(file, name, s.Value)
		},
	})
	require.NoError(t, err)
	_, err = d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
	require.NoError(t, err)
	press := func(keys ...string) {
		for _, k := range keys {
			require.NoError(t, d.Press(k))
			require.NoError(t, d.Run(2))
		}
	}

	// s opens the settings, on their one row; Enter opens the Keys panel.
	press("s", "Enter")
	require.Equal(t, 1, d.Shown().Overlays, "the Keys panel should be open")
	// Walk to the timeline's row, record y for it, and go back to the map.
	for d.FocusedBinding() != "timeline" {
		press("ArrowDown")
		require.Less(t, d.Steps(), 200, "no timeline row in the Keys panel")
	}
	press("Enter", "y", "Escape", "Escape")

	t.Run("when the timeline is rebound to y in the Keys panel", func(t *testing.T) {
		t.Run("and y is pressed at the map", func(t *testing.T) {
			press("y")

			t.Run("it should open the timeline", func(t *testing.T) {
				assert.True(t, d.Shown().Timeline)
			})
		})

		t.Run("it should write it to the config file", func(t *testing.T) {
			text, err := os.ReadFile(file)
			require.NoError(t, err)
			assert.Contains(t, string(text), "timeline = 'y'")
		})
	})
}
