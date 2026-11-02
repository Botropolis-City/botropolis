//go:build linux

package acceptance_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

func TestWaitingRecord(t *testing.T) {
	t.Run("when a background session's record says waiting for a permission prompt and its transcript ends in an unanswered tool call", func(t *testing.T) {
		const (
			blocked = "331a0000-0000-4000-8000-000000000331"
			control = "331b0000-0000-4000-8000-000000000331"
			cwd     = "/home/user/src/city"
		)
		daemon, err := botropolisd()
		require.NoError(t, err)
		home := helpers.NewHome(t)
		at := time.Now().Add(-8 * time.Hour).UTC().Format(time.RFC3339Nano)
		for _, sid := range []string{blocked, control} {
			child := exec.Command("sleep", "600")
			require.NoError(t, child.Start())
			t.Cleanup(func() { _ = child.Process.Kill(); _, _ = child.Process.Wait() })
			if sid == blocked {
				home.WaitingSession(child.Process.Pid, sid, cwd, "bg", "permission prompt")
			} else {
				home.Session(child.Process.Pid, sid, cwd, "bg", "busy")
			}
			home.Transcript(sid, cwd, helpers.UserPrompt(sid, cwd, at),
				helpers.AssistantToolCall(sid, "msg_"+sid[:4], at, "toolu_"+sid[:4], "Bash"))
		}
		root := t.TempDir()
		sock := shortSocket(t)
		config := filepath.Join(root, "config.toml")
		d := exec.Command(daemon, "--home", home.Path, "--socket", sock, "--config", config)
		d.Env = append(os.Environ(), "XDG_STATE_HOME="+filepath.Join(root, "state"), "PATH=/nonexistent")
		require.NoError(t, d.Start())
		t.Cleanup(func() { _ = d.Process.Kill(); _ = d.Wait() })

		states := func() map[string]state.State {
			out, err := exec.Command(botropolis, "status", "--all", "--json", "--socket", sock, "--config", config).Output()
			if err != nil {
				return nil
			}
			var sessions []state.Session
			if json.Unmarshal(out, &sessions) != nil {
				return nil
			}
			found := map[string]state.State{}
			for _, s := range sessions {
				found[s.ID] = s.State
			}
			return found
		}
		require.Eventually(t, func() bool { return states()[control] == state.Unattended }, time.Minute, 100*time.Millisecond,
			"the daemon never served the control session as unattended")

		t.Run("status --json should show it as needs you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, states()[blocked])
		})
	})
}
