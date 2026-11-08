package state_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

// Item 365: the loader names the files it reads outside the home, so the
// daemon watches them too and a move written to the groups file takes on
// a quiet home.
func TestLoaderWatchFiles(t *testing.T) {
	t.Run("when the loader keeps groups", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		loader := state.NewLoader(t.TempDir(), dead).WithGroups(path)

		t.Run("it should name the groups file", func(t *testing.T) {
			assert.Equal(t, []string{path}, loader.WatchFiles())
		})
	})

	t.Run("when it keeps none", func(t *testing.T) {
		loader := state.NewLoader(t.TempDir(), dead)

		t.Run("it should name nothing", func(t *testing.T) {
			assert.Empty(t, loader.WatchFiles())
		})
	})
}
