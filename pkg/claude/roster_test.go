package claude_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rosterJSON = `{"proto":1,"supervisorPid":1646593,"updatedAt":1789617482735,"workers":{` +
	`"0898d7e4":{"pid":1646612,"procStart":"2189863","sessionId":"0898d7e4-f2cb-45fe-833e-1921c1f4bcaa",` +
	`"rendezvousSock":"/tmp/cc-daemon-1000/ffe95242/rv/0898d7e4.sock",` +
	`"ptySock":"/tmp/cc-daemon-1000/ffe95242/pty/0898d7e4.sock","cliVersion":"2.1.273",` +
	`"startedAt":1789617471534,"attempt":1,"cwd":"/home/avesta/workspaces/github/auroq/botropolis",` +
	`"rvAuth":"<secret>","ptyAuth":"<secret>","replPid":1646630,"replProcStart":"2189871"}}}`

func TestReadRoster(t *testing.T) {
	t.Run("when the roster holds one worker", func(t *testing.T) {
		roster, err := claude.ReadRoster(writeFile(t, t.TempDir(), "roster.json", rosterJSON))
		require.NoError(t, err)
		worker, ok := roster["0898d7e4"]
		require.True(t, ok)

		t.Run("it should key the worker by job id", func(t *testing.T) {
			assert.Equal(t, "0898d7e4", worker.JobID)
		})

		t.Run("it should read the session id", func(t *testing.T) {
			assert.Equal(t, "0898d7e4-f2cb-45fe-833e-1921c1f4bcaa", worker.SessionID)
		})

		t.Run("it should read the repl pid", func(t *testing.T) {
			assert.Equal(t, 1646630, worker.PID)
		})

		t.Run("it should read the pty socket path", func(t *testing.T) {
			assert.Equal(t, "/tmp/cc-daemon-1000/ffe95242/pty/0898d7e4.sock", worker.PtySock)
		})

		t.Run("it should read the cwd", func(t *testing.T) {
			assert.Equal(t, "/home/avesta/workspaces/github/auroq/botropolis", worker.CWD)
		})
	})

	t.Run("when the roster does not exist", func(t *testing.T) {
		roster, err := claude.ReadRoster(filepath.Join(t.TempDir(), "roster.json"))

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return no workers", func(t *testing.T) {
			assert.Empty(t, roster)
		})
	})

	t.Run("when the roster is not valid JSON", func(t *testing.T) {
		_, err := claude.ReadRoster(writeFile(t, t.TempDir(), "roster.json", `{"workers":`))

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
