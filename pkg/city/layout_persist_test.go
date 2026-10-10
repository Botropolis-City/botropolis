package city_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLayoutPersistence(t *testing.T) {
	t.Run("when a layout is saved and loaded again", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state", "botropolis", "layout.json")
		layout := city.NewLayout()
		first := build(t, layout, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		require.NoError(t, layout.Save(path))
		reloaded, err := city.LoadLayout(path)
		require.NoError(t, err)
		second := city.Build(snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working)), reloaded)

		t.Run("it should place every district where it was", func(t *testing.T) {
			assert.Equal(t, first.Districts[0].Rect, second.Districts[0].Rect)
		})

		t.Run("it should place every building where it was", func(t *testing.T) {
			assert.Equal(t, first.Buildings()[0].Rect, second.Buildings()[0].Rect)
		})

		t.Run("it should write the file privately", func(t *testing.T) {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	})

	t.Run("when a layout remembers when it was last seen", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		layout := city.NewLayout()
		layout.Seen = time.Date(2026, time.September, 18, 20, 0, 0, 0, time.UTC)
		require.NoError(t, layout.Save(path))
		reloaded, err := city.LoadLayout(path)
		require.NoError(t, err)

		t.Run("it should round-trip the moment", func(t *testing.T) {
			assert.True(t, reloaded.Seen.Equal(layout.Seen))
		})
	})

	t.Run("when a layout was never seen", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		require.NoError(t, city.NewLayout().Save(path))
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		t.Run("it should leave the moment out of the file", func(t *testing.T) {
			assert.NotContains(t, string(data), "seen")
		})
	})

	t.Run("when the layout file does not exist", func(t *testing.T) {
		layout, err := city.LoadLayout(filepath.Join(t.TempDir(), "missing.json"))

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return an empty layout", func(t *testing.T) {
			require.NotNil(t, layout)
			assert.Empty(t, layout.Districts)
		})
	})

	t.Run("when the layout file is from an older version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version":0,"districts":{"/p":{"slot":9,"slots":{}}}}`), 0o600))
		layout, err := city.LoadLayout(path)
		require.NoError(t, err)

		t.Run("it should start over rather than trust it", func(t *testing.T) {
			assert.Empty(t, layout.Districts)
		})
	})

	t.Run("when the layout file is not valid JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version":`), 0o600))
		_, err := city.LoadLayout(path)

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when XDG_STATE_HOME is set", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/tmp/state")

		t.Run("it should put the layout under it", func(t *testing.T) {
			assert.Equal(t, "/tmp/state/botropolis/layout.json", city.LayoutPath())
		})
	})

	t.Run("when XDG_STATE_HOME is not set", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		t.Setenv("HOME", "/home/someone")

		t.Run("it should fall back to ~/.local/state", func(t *testing.T) {
			assert.Equal(t, "/home/someone/.local/state/botropolis/layout.json", city.LayoutPath())
		})
	})
}
