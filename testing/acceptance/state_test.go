package acceptance_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
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

// command is the city in this run's environment: headless, it brings
// its own display; otherwise it gets a private one for this test.
func (r stateRun) command(t *testing.T, headless bool, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(botropolis, append([]string{"city", "--socket", filepath.Join(r.tmp, "none.sock")}, args...)...)
	cmd.Env = r.env()
	if !headless {
		cmd.Env = append(cmd.Env, helpers.Display(t, render.DefaultWidth, render.DefaultHeight).Env()...)
	}
	return cmd
}

func (r stateRun) city(t *testing.T, args ...string) {
	t.Helper()
	out, err := r.command(t, containsArg(args, "--headless"), args...).CombinedOutput()
	require.NoError(t, err, string(out))
}

// interruption is one way a city is signalled: the run, the signal,
// and which process it reaches.
type interruption struct {
	name     string
	headless bool
	sig      syscall.Signal
	// to picks what to signal from the process the test started and the
	// city's own; a negative pid is a process group.
	to func(started, city int) int
}

func toCity(_, city int) int        { return city }
func toStarted(started, _ int) int  { return started }
func toTerminal(started, _ int) int { return -started }

// interrupted is what an interruption left behind.
type interrupted struct {
	err          error
	citiesLeft   int
	siblingAlive bool
}

// interrupt opens the city in a process group of its own, standing for
// a terminal's, beside a sibling in that group that ignores SIGINT and
// SIGTERM; waits for its private state directory and its window; sends
// the signal; and returns what was left once it exited.
func (r stateRun) interrupt(t *testing.T, how interruption, args ...string) interrupted {
	t.Helper()
	if how.headless {
		args = append(args, "--headless")
	}
	cmd := r.command(t, how.headless, args...)
	cmd.Env = append(cmd.Env, "BOTROPOLIS_FRAMETIME=1")
	var out syncBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	sibling := exec.Command("sh", "-c", `trap "" INT TERM; exec sleep 300`)
	sibling.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: cmd.Process.Pid}
	require.NoError(t, sibling.Start())
	siblingDone := make(chan struct{})
	go func() { _ = sibling.Wait(); close(siblingDone) }()
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = sibling.Process.Kill()
		for _, pid := range cities(r.tmp) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})

	deadline := time.Now().Add(60 * time.Second)
	city := 0
	for city == 0 {
		matches, _ := filepath.Glob(filepath.Join(r.tmp, "botropolis-state-*"))
		if len(matches) > 0 && how.headless {
			city = descendant(cmd.Process.Pid, "botropolis")
		} else if len(matches) > 0 {
			city = cmd.Process.Pid
		}
		select {
		case err := <-exited:
			t.Fatalf("the city exited before it was signalled: %v\n%s", err, out.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the city made no private state directory in a minute\n%s", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	// The signal is for a city whose loop is running, which says so with
	// its first FRAME line (item 226). The city also stops cleanly on a
	// signal that comes before its loop, so no delay makes this wait's
	// absence fail; it keeps the test on the path it is named for.
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "FRAME ") }, time.Minute, 50*time.Millisecond,
		"the city's loop never reported a frame\n%s", out.String())
	require.NoError(t, syscall.Kill(how.to(cmd.Process.Pid, city), how.sig))

	var left interrupted
	select {
	case err := <-exited:
		if err != nil {
			left.err = errors.New(err.Error() + "\n" + out.String())
		}
	case <-time.After(30 * time.Second):
		left.err = errors.New("still running 30 s after " + how.name + "\n" + out.String())
	}
	left.citiesLeft = len(cities(r.tmp))
	select {
	case <-siblingDone:
	default:
		left.siblingAlive = true
	}
	return left
}

// cities finds every city process still running whose command line
// names dir.
func cities(dir string) []int {
	var out []int
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		cmdline, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		stat, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		zombie := strings.Contains(string(stat), ") Z ")
		if strings.TrimSpace(string(comm)) == "botropolis" && strings.Contains(string(cmdline), dir) && !zombie {
			out = append(out, pid)
		}
	}
	return out
}

// descendant finds the process named name below root, by walking /proc.
func descendant(root int, name string) int {
	entries, _ := os.ReadDir("/proc")
	parent := map[int]int{}
	comm := map[int]string{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		open, shut := strings.IndexByte(string(stat), '('), strings.LastIndexByte(string(stat), ')')
		if open < 0 || shut < open {
			continue
		}
		fields := strings.Fields(string(stat[shut+1:]))
		if len(fields) < 2 {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		parent[pid], comm[pid] = ppid, string(stat[open+1:shut])
	}
	for pid, c := range comm {
		if c != name {
			continue
		}
		for p := parent[pid]; p > 1; p = parent[p] {
			if p == root {
				return pid
			}
		}
	}
	return 0
}

// env is this run's environment: its own HOME, state and TMPDIR, and no
// display of the person's own.
func (r stateRun) env() []string {
	env := []string{"HOME=" + r.home, "XDG_STATE_HOME=" + r.realState, "TMPDIR=" + r.tmp}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "HOME", "XDG_STATE_HOME", "TMPDIR", "DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY":
		default:
			env = append(env, kv)
		}
	}
	return env
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
	helpers.NeedXvfb(t)

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

	interruptions := []interruption{
		{name: "a --home city is sent SIGINT", sig: syscall.SIGINT, to: toCity},
		{name: "a --home city is sent SIGTERM", sig: syscall.SIGTERM, to: toCity},
		{name: "a --headless city's terminal sends SIGINT", headless: true, sig: syscall.SIGINT, to: toTerminal},
		{name: "a --headless city is sent SIGTERM", headless: true, sig: syscall.SIGTERM, to: toStarted},
	}
	for _, how := range interruptions {
		t.Run("when "+how.name, func(t *testing.T) {
			r := newStateRun(t)
			other := filepath.Join(r.tmp, "other-home")
			withSession(t, other)
			left := r.interrupt(t, how, "--home", other)

			t.Run("it should exit cleanly", func(t *testing.T) {
				assert.NoError(t, left.err)
			})

			t.Run("it should leave no private state directory behind", func(t *testing.T) {
				matches, _ := filepath.Glob(filepath.Join(r.tmp, "botropolis-state-*"))
				assert.Empty(t, matches)
			})

			t.Run("it should leave no city running", func(t *testing.T) {
				assert.Zero(t, left.citiesLeft)
			})

			t.Run("it should leave the terminal's own process group untouched", func(t *testing.T) {
				assert.True(t, left.siblingAlive)
			})
		})
	}

	t.Run("when a city that died left its private state directory", func(t *testing.T) {
		r := newStateRun(t)
		gone := exec.Command("true")
		require.NoError(t, gone.Run())
		left := filepath.Join(r.tmp, "botropolis-state-left")
		require.NoError(t, os.MkdirAll(left, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(left, "owner"), []byte(strconv.Itoa(gone.Process.Pid)+" 1\n"), 0o600))
		other := filepath.Join(r.tmp, "other-home")
		withSession(t, other)
		r.city(t, "--home", other, "--screenshot", filepath.Join(r.tmp, "shot.png"))

		t.Run("it should be removed by the next run that makes one", func(t *testing.T) {
			assert.NoDirExists(t, left)
		})
	})

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
