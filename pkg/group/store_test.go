package group_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Run("when the state directory is set", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_STATE_HOME", dir)

		t.Run("it should keep groups in botropolis's own state", func(t *testing.T) {
			assert.Equal(t, filepath.Join(dir, "botropolis", "groups.json"), group.Path())
		})
	})

	t.Run("when memory is saved and loaded", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "botropolis", "groups.json")
		saved := group.Memory{
			Groups:     []group.Stored{{ID: "g1", Name: "Headers", Renamed: true, Anchor: "PROJ-123", Members: []string{"a", "b"}}},
			NotRelated: []group.Pair{{"a", "c"}},
			Muted:      []string{"headers"},
		}
		require.NoError(t, group.Save(path, saved))
		loaded, err := group.Load(path)
		require.NoError(t, err)

		t.Run("it should round-trip groups, not-related pairs and muted words", func(t *testing.T) {
			assert.Equal(t, saved, loaded)
		})
	})

	t.Run("when the file is missing", func(t *testing.T) {
		loaded, err := group.Load(filepath.Join(t.TempDir(), "groups.json"))
		require.NoError(t, err)

		t.Run("it should load empty", func(t *testing.T) {
			assert.Equal(t, group.Memory{}, loaded)
		})
	})

	t.Run("when a detected result is remembered", func(t *testing.T) {
		prompt := "add rate-limit headers to the limiter for PROJ-123"
		result := detect(group.Memory{},
			member("a", "/w/lib", 0, prompt),
			member("b", "/w/api", 3*time.Hour, "PROJ-123: read the rate-limit headers"))
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{}.Remember(result)))
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		t.Run("it should remember the group", func(t *testing.T) {
			loaded, _ := group.Load(path)
			assert.Equal(t, []string{"a", "b"}, loaded.Groups[0].Members)
		})

		t.Run("it should never write prompt text", func(t *testing.T) {
			assert.NotContains(t, string(data), prompt)
		})
	})
}
