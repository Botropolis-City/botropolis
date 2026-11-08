package harness_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/harness"
)

type filesFake struct {
	fake
	files []string
}

func (f filesFake) WatchFiles() []string { return f.files }

// Item 365: the daemon is handed the harnesses as one, so the files any
// of them names reach its watcher.
func TestMultiWatchFiles(t *testing.T) {
	t.Run("when one harness names files and another cannot", func(t *testing.T) {
		m := harness.NewMulti(filesFake{fake: fake{name: "claude"}, files: []string{"/state/groups.json"}}, fake{name: "codex"})

		t.Run("it should name that harness's files", func(t *testing.T) {
			assert.Equal(t, []string{"/state/groups.json"}, m.WatchFiles())
		})
	})
}
