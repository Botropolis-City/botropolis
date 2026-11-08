package daemon_test

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
)

// countingLoader is the loader, counting the rescans that load it.
type countingLoader struct {
	*state.Loader
	loads atomic.Int32
}

func (c *countingLoader) Load(at time.Time) (state.Snapshot, error) {
	c.loads.Add(1)
	return c.Loader.Load(at)
}

// Item 365: the city writes a move into the groups file and tells the
// daemon nothing, so the daemon watches that file: on a quiet home,
// where nothing in ~/.claude changes after the move, it used never to
// take.
func TestWatchGroups(t *testing.T) {
	const quiet = 500 * time.Millisecond
	start := func(t *testing.T, path string) (*daemon.Daemon, *countingLoader) {
		t.Helper()
		home := idleHome(t, "interactive")
		loader := &countingLoader{Loader: state.NewLoader(home.Path, probes).WithGroups(path)}
		d := daemon.NewWith(loader, func() time.Time { return now })
		require.NoError(t, d.Rescan())
		watch(t, d)
		return d, loader
	}
	moved := func(t *testing.T, path string) string {
		t.Helper()
		memory, id := group.Memory{}.WithNewGroup("Gateway", sid)
		require.NoError(t, group.Save(path, memory.WithMove(sid, id)))
		return id
	}
	inGroup := func(d *daemon.Daemon, id string) func() bool {
		return func() bool {
			s, _ := session(d, sid)
			return s.Group == id
		}
	}
	named := func(d *daemon.Daemon, id, name string) func() bool {
		return func() bool {
			for _, g := range d.Snapshot().Groups {
				if g.ID == id {
					return g.Name == name
				}
			}
			return false
		}
	}

	t.Run("when the groups file changes on a quiet home", func(t *testing.T) {
		t.Run("and it moves a session into a group", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "groups.json")
			d, _ := start(t, path)
			id := moved(t, path)

			t.Run("it should show the session in that group", func(t *testing.T) {
				assert.Eventually(t, inGroup(d, id), settle, tick)
			})
		})

		t.Run("and it renames a group", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "groups.json")
			id := moved(t, path)
			d, _ := start(t, path)
			memory, err := group.Load(path)
			require.NoError(t, err)
			require.NoError(t, group.Save(path, memory.WithRename(id, "Gatehouse")))

			t.Run("it should show the new name", func(t *testing.T) {
				assert.Eventually(t, named(d, id, "Gatehouse"), settle, tick)
			})
		})
	})

	t.Run("when a file beside the groups file changes", func(t *testing.T) {
		dir := t.TempDir()
		_, loader := start(t, filepath.Join(dir, "groups.json"))
		before := loader.loads.Load()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "stats.db"), []byte("written"), 0o600))
		time.Sleep(quiet)

		t.Run("it should not rescan", func(t *testing.T) {
			assert.Equal(t, before, loader.loads.Load())
		})
	})

	t.Run("when the groups file's directory does not exist yet", func(t *testing.T) {
		t.Run("and the groups file is then written", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state", "botropolis", "groups.json")
			d, _ := start(t, path)
			id := moved(t, path)

			t.Run("it should show the move", func(t *testing.T) {
				assert.Eventually(t, inGroup(d, id), settle, tick)
			})
		})
	})
}
