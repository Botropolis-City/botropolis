package commands_test

import (
	"bytes"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvents(t *testing.T) {
	t.Run("when the daemon holds events", func(t *testing.T) {
		d, sock := servedDaemon(t)
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Grep"}, daemonNow)
		d.Apply(claude.HookEvent{Name: claude.HookStop, SessionID: sid}, daemonNow.Add(time.Minute))
		var out bytes.Buffer
		require.NoError(t, commands.NewEvents(sock).Run(&out, daemonT0))

		t.Run("it should print one row per event with its time, kind and title", func(t *testing.T) {
			assert.Equal(t, daemonNow.Local().Format("15:04")+"  needs-you   Fix the CI queue\n", out.String())
		})

		t.Run("and since is after everything", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, commands.NewEvents(sock).Run(&out, daemonNow))

			t.Run("it should say so", func(t *testing.T) {
				assert.Equal(t, "no events\n", out.String())
			})
		})
	})

	t.Run("when no daemon is listening", func(t *testing.T) {
		err := commands.NewEvents(t.TempDir()+"/none.sock").Run(&bytes.Buffer{}, time.Time{})

		t.Run("it should say the daemon is needed", func(t *testing.T) {
			assert.ErrorContains(t, err, "daemon")
		})
	})
}
