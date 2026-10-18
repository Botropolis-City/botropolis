package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// stateRun is one run of the city with its own HOME, XDG_STATE_HOME and
// TMPDIR, so what it writes to the person's own state, and what private
// state it leaves behind, can be seen.
type stateRun struct {
	home, realState, tmp string
}

func newStateRun(t *testing.T) stateRun {
	t.Helper()
	root := t.TempDir()
	r := stateRun{home: filepath.Join(root, "home"), realState: filepath.Join(root, "state"), tmp: filepath.Join(root, "tmp")}
	for _, dir := range []string{r.home, r.realState, r.tmp, filepath.Join(r.tmp, "frames")} {
		require.NoError(t, os.MkdirAll(dir, 0o700))
	}
	return r
}

func withSession(t *testing.T, home string) {
	h := helpers.HomeAt(t, home)
	id := "11111111-0000-0000-0000-000000000111"
	h.Transcript(id, "/w/lib", helpers.UserPrompt(id, "/w/lib", "2026-10-18T08:00:00.000Z"), helpers.AssistantReply(id, "msg_1111", "2026-10-18T08:00:00.000Z"))
}

func (r stateRun) city(t *testing.T, args ...string) {
	t.Helper()
	full := append([]string{"city", "--socket", filepath.Join(r.tmp, "none.sock")}, args...)
	var cmd *exec.Cmd
	if containsArg(args, "--headless") {
		cmd = exec.Command(botropolis, full...)
	} else {
		cmd = exec.Command("xvfb-run", append([]string{"-a", botropolis}, full...)...)
	}
	env := []string{"HOME=" + r.home, "XDG_STATE_HOME=" + r.realState, "TMPDIR=" + r.tmp}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "HOME", "XDG_STATE_HOME", "TMPDIR", "DISPLAY", "WAYLAND_DISPLAY":
		default:
			env = append(env, kv)
		}
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func names(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && path != dir {
			out = append(out, strings.TrimPrefix(path, dir+string(filepath.Separator)))
		}
		return nil
	})
	return out
}

// Item 111: a run that is not the person's own city never touches their
// state.
func TestFrameRunsKeepTheirOwnState(t *testing.T) {
	if _, err := exec.LookPath("xvfb-run"); err != nil {
		t.Skip("no xvfb-run to open the city on a virtual display")
	}

	t.Run("when the city runs with only --screenshot", func(t *testing.T) {
		r := newStateRun(t)
		withSession(t, r.home)
		r.city(t, "--screenshot", filepath.Join(r.tmp, "shot.png"))

		t.Run("it should write layout.json into the real state directory", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(r.realState, "botropolis", "layout.json"))
		})
	})

	private := []struct {
		name string
		args func(r stateRun) []string
	}{
		{"--headless", func(r stateRun) []string {
			return []string{"--headless", "--screenshot", filepath.Join(r.tmp, "shot.png")}
		}},
		{"--record", func(r stateRun) []string {
			return []string{"--record", filepath.Join(r.tmp, "frames"), "--seconds", "0.3"}
		}},
		{"--script", func(r stateRun) []string {
			script := filepath.Join(r.tmp, "cues.json")
			_ = os.WriteFile(script, []byte(`[{"at":0.1,"key":"n"}]`), 0o600)
			return []string{"--record", filepath.Join(r.tmp, "frames"), "--seconds", "0.3", "--script", script}
		}},
		{"--home", func(r stateRun) []string {
			other := filepath.Join(r.tmp, "other-home")
			withSession(t, other)
			return []string{"--home", other, "--screenshot", filepath.Join(r.tmp, "shot.png")}
		}},
	}
	for _, p := range private {
		t.Run("when the city runs with "+p.name, func(t *testing.T) {
			r := newStateRun(t)
			withSession(t, r.home)
			r.city(t, p.args(r)...)

			t.Run("it should write nothing under the real state directory", func(t *testing.T) {
				assert.Empty(t, names(r.realState))
			})

			t.Run("it should leave no private state directory behind", func(t *testing.T) {
				matches, _ := filepath.Glob(filepath.Join(r.tmp, "botropolis-state-*"))
				assert.Empty(t, matches)
			})
		})
	}

	t.Run("when a --home run's home holds a seeded layout", func(t *testing.T) {
		r := newStateRun(t)
		other := filepath.Join(r.tmp, "other-home")
		withSession(t, other)
		seeded := filepath.Join(other, ".local", "state", "botropolis", "layout.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(seeded), 0o700))
		require.NoError(t, os.WriteFile(seeded, []byte(`{"version":1}`), 0o600))
		r.city(t, "--home", other, "--screenshot", filepath.Join(r.tmp, "shot.png"))
		data, _ := os.ReadFile(seeded)

		t.Run("it should leave the seeded file unchanged", func(t *testing.T) {
			assert.Equal(t, `{"version":1}`, string(data))
		})
	})
}
