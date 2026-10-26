//go:build linux

package acceptance_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

var botropolisHook = sync.OnceValues(func() (string, error) {
	bin := filepath.Join(filepath.Dir(botropolis), "botropolis-hook")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/botropolis-hook").CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return bin, nil
})

func TestShellRecord(t *testing.T) {
	t.Run("when the daemon has the Stop hook for a session whose record says shell", func(t *testing.T) {
		const (
			shell   = "250a0000-0000-4000-8000-000000000250"
			control = "250b0000-0000-4000-8000-000000000250"
			cwd     = "/home/user/src/city"
		)
		daemon, err := botropolisd()
		require.NoError(t, err)
		hook, err := botropolisHook()
		require.NoError(t, err)
		home := helpers.NewHome(t)
		at := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
		for _, s := range []struct{ sid, status string }{{shell, "shell"}, {control, "busy"}} {
			child := exec.Command("sleep", "600")
			require.NoError(t, child.Start())
			t.Cleanup(func() { _ = child.Process.Kill(); _, _ = child.Process.Wait() })
			home.Session(child.Process.Pid, s.sid, cwd, "interactive", s.status)
			home.Transcript(s.sid, cwd, helpers.UserPrompt(s.sid, cwd, at), helpers.AssistantReply(s.sid, "msg_"+s.sid[:4], at))
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
		require.Eventually(t, func() bool { return len(states()) == 2 }, time.Minute, 100*time.Millisecond, "the daemon never served both sessions")
		for _, sid := range []string{shell, control} {
			send := exec.Command(hook, "--socket", sock, "--config", config)
			send.Stdin = strings.NewReader(fmt.Sprintf(`{"hook_event_name":"Stop","session_id":%q,"cwd":%q}`, sid, cwd))
			out, err := send.CombinedOutput()
			require.NoError(t, err, string(out))
		}
		require.Eventually(t, func() bool { return states()[control] == state.NeedsYou }, time.Minute, 100*time.Millisecond,
			"the control session's Stop hook never reached the daemon")

		t.Run("status --json should not show it as needs you", func(t *testing.T) {
			assert.Equal(t, state.Working, states()[shell])
		})
	})
}
