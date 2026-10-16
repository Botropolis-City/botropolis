package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

func TestFind(t *testing.T) {
	t.Run("when run with -n against a sample home and no daemon", func(t *testing.T) {
		home := helpers.NewHome(t)
		for i := range 3 {
			id := fmt.Sprintf("0000000%d-find-acceptance", i)
			home.Transcript(id, "/src/kiln", fmt.Sprintf(
				`{"type":"user","timestamp":"2026-10-16T09:0%d:00.000Z","sessionId":%q,"cwd":"/src/kiln","message":{"role":"user","content":"why does the kiln overheat"}}`, i, id))
		}
		home.Transcript("00000009-find-acceptance", "/src/orchard",
			`{"type":"user","timestamp":"2026-10-16T09:09:00.000Z","sessionId":"00000009-find-acceptance","cwd":"/src/orchard","message":{"role":"user","content":"prune the orchard"}}`)
		stateHome := t.TempDir()
		cmd := exec.Command(botropolis, "find", "-n", "2", "--home", home.Path,
			"--socket", filepath.Join(t.TempDir(), "none.sock"), "kiln overheat")
		cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+stateHome)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))

		t.Run("it should keep no ledger of a home it was pointed at", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(stateHome, "botropolis", "sessions.json"))
		})

		t.Run("it should print at most n matching sessions", func(t *testing.T) {
			var sessions []string
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if !strings.HasPrefix(line, " ") {
					sessions = append(sessions, line)
				}
			}
			assert.Len(t, sessions, 2, string(out))
		})
	})
}
