package appd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 87: groups are the default home's. A run pointed at another home,
// a fixture or a test's, never reads or writes them.
func TestGroupsStayWithTheDefaultHome(t *testing.T) {
	now := time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC)
	dead := state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
	grouped := func(t *testing.T) string {
		t.Helper()
		home := helpers.NewHome(t)
		for _, s := range []struct{ id, dir, at string }{
			{"11111111-0000-0000-0000-000000000000", "/w/lib", "2026-09-16T10:00:00.000Z"},
			{"22222222-0000-0000-0000-000000000000", "/w/api", "2026-09-16T14:00:00.000Z"},
		} {
			home.Transcript(s.id, s.dir, helpers.UserPrompt(s.id, s.dir, s.at), helpers.AssistantReply(s.id, "msg_"+s.id[:4], s.at))
			at, _ := time.Parse(time.RFC3339, s.at)
			home.Prompt(s.id, s.dir, "PROJ-123 headers", at)
		}
		return home.Path
	}
	loaded := func(t *testing.T, cfg *config.Config) []os.DirEntry {
		t.Helper()
		stateDir := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateDir)
		_, err := Harnesses(cfg, dead, Paced).Load(now)
		require.NoError(t, err)
		entries, _ := os.ReadDir(filepath.Join(stateDir, "botropolis"))
		return entries
	}

	t.Run("when run with --home", func(t *testing.T) {
		entries := loaded(t, &config.Config{Home: grouped(t), HomeSet: true})

		t.Run("it should not touch the default state directory", func(t *testing.T) {
			assert.Empty(t, entries)
		})
	})

	t.Run("when run against the default home", func(t *testing.T) {
		entries := loaded(t, &config.Config{Home: grouped(t)})

		t.Run("it should remember its groups there", func(t *testing.T) {
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			assert.Contains(t, names, "groups.json")
		})
	})
}
