package acceptance_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 186. Fourteen item sessions each ran make lint && make test at once
// on one machine, and a stopgap lock taken through sh -c could not run from
// a worktree session. make land takes the lock itself, so a session runs one
// plain command. These tests run the real Makefile in a scratch repository
// whose "github" remote is a bare repository, with probe gates in place of
// lint and test.
const landProbes = `
land-probe-slow ::
	@echo "${PROBE_NAME} start" >> ${PROBE_LOG}
	@sleep 3
	@echo "${PROBE_NAME} end" >> ${PROBE_LOG}

land-probe-fail ::
	@exit 1

land-probe-hang ::
	@echo "${PROBE_NAME} start" >> ${PROBE_LOG}
	@sleep 600
`

type landRig struct {
	dir    string
	remote string
	lock   string
	log    string
	env    []string
}

func newLandRig(t *testing.T) *landRig {
	t.Helper()
	dir := t.TempDir()
	r := &landRig{
		dir:    dir,
		remote: filepath.Join(dir, "remote.git"),
		lock:   filepath.Join(dir, "land.lock"),
		log:    filepath.Join(dir, "probe.log"),
	}
	// make land asks gh for main's CI (item 202); a stand-in answers green
	// so the rig never reaches GitHub.
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/usr/bin/env bash\necho success 0000000 https://example.com/run\n"), 0o755))
	r.env = append(landEnviron(),
		"PATH="+bin+string(filepath.ListSeparator)+os.Getenv("PATH"),
		"GOFLAGS=",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"LAND_LOCK="+r.lock,
		"PROBE_LOG="+r.log,
	)
	r.git(t, dir, "init", "-q", "--bare", "-b", "main", r.remote)
	seed := filepath.Join(dir, "seed")
	root := repoRoot(t)
	for _, name := range []string{"Makefile", "go.mod", "VERSION", "tools/lint/go.mod", "tools/ci-status"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		info, err := os.Stat(filepath.Join(root, name))
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(seed, filepath.Dir(name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(seed, name), data, info.Mode().Perm()))
	}
	appendTo(t, filepath.Join(seed, "Makefile"), landProbes)
	require.NoError(t, os.WriteFile(filepath.Join(seed, "shared.txt"), []byte("base\n"), 0o644))
	r.git(t, seed, "init", "-q", "-b", "main")
	r.git(t, seed, "add", ".")
	r.git(t, seed, "commit", "-q", "-m", "seed")
	r.git(t, seed, "remote", "add", "github", r.remote)
	r.git(t, seed, "push", "-q", "github", "main")
	return r
}

// landEnviron is this process's environment without an outer make's flags,
// since make test runs this test under make.
func landEnviron() []string {
	var env []string
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "MAKEFLAGS", "MFLAGS", "MAKELEVEL", "GOFLAGS":
			continue
		}
		env = append(env, kv)
	}
	return env
}

func (r *landRig) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// session clones the remote and commits one change, as an item session's
// worktree would hold.
func (r *landRig) session(t *testing.T, name, file, text string) string {
	t.Helper()
	dir := filepath.Join(r.dir, name)
	r.git(t, r.dir, "clone", "-q", "-o", "github", r.remote, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(text), 0o644))
	r.git(t, dir, "add", file)
	r.git(t, dir, "commit", "-q", "-m", name)
	return dir
}

func (r *landRig) land(dir, name, gate string, extra ...string) *exec.Cmd {
	cmd := exec.Command("make", "-s", "-C", dir, "land", "LAND_GATE="+gate)
	cmd.Env = append(append(append([]string{}, r.env...), "PROBE_NAME="+name), extra...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func (r *landRig) remoteMain(t *testing.T) string {
	t.Helper()
	return r.git(t, r.dir, "--git-dir", r.remote, "rev-parse", "main")
}

func (r *landRig) remoteSubjects(t *testing.T) []string {
	t.Helper()
	return strings.Split(r.git(t, r.dir, "--git-dir", r.remote, "log", "--no-show-signature", "--format=%s", "main"), "\n")
}

func (r *landRig) probeLog(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(r.log)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return strings.Fields(strings.ReplaceAll(string(data), " ", "_"))
}

func (r *landRig) waitForStart(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		for _, line := range r.probeLog(t) {
			if line == name+"_start" {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s's gate never started", name)
}

func TestLand(t *testing.T) {
	if testing.Short() {
		t.Skip("runs make land in scratch repositories")
	}
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("flock is not installed, so make land went untested")
	}
	t.Parallel()

	t.Run("when two sessions run make land at once, and the gate takes a moment", func(t *testing.T) {
		t.Parallel()
		r := newLandRig(t)
		a := r.session(t, "a", "a.txt", "a\n")
		b := r.session(t, "b", "b.txt", "b\n")

		first := r.land(a, "a", "land-probe-slow")
		firstOut, firstErr := runAsync(first)
		r.waitForStart(t, "a")
		secondOut, err := r.land(b, "b", "land-probe-slow").CombinedOutput()
		require.NoError(t, err, string(secondOut))
		require.NoError(t, <-firstErr, firstOut.String())

		t.Run("it should run their gates one after the other", func(t *testing.T) {
			assert.Equal(t, []string{"a_start", "a_end", "b_start", "b_end"}, r.probeLog(t))
		})

		t.Run("it should tell the second it is waiting", func(t *testing.T) {
			assert.Contains(t, string(secondOut), "Waiting for the landing in progress")
		})

		t.Run("it should land both commits on the remote's main", func(t *testing.T) {
			assert.Equal(t, []string{"b", "a", "seed"}, r.remoteSubjects(t))
		})
	})

	t.Run("when the gate fails", func(t *testing.T) {
		t.Parallel()
		r := newLandRig(t)
		before := r.remoteMain(t)
		a := r.session(t, "a", "a.txt", "a\n")
		out, err := r.land(a, "a", "land-probe-fail").CombinedOutput()

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.Error(t, err, string(out))
		})

		t.Run("it should leave the remote's main where it was", func(t *testing.T) {
			assert.Equal(t, before, r.remoteMain(t))
		})
	})

	t.Run("when the session holding the lock is killed mid-gate", func(t *testing.T) {
		t.Parallel()
		r := newLandRig(t)
		a := r.session(t, "a", "a.txt", "a\n")
		b := r.session(t, "b", "b.txt", "b\n")
		holder := r.land(a, "a", "land-probe-hang")
		holderOut, holderErr := runAsync(holder)
		r.waitForStart(t, "a")
		require.NoError(t, syscall.Kill(-holder.Process.Pid, syscall.SIGKILL))
		require.Error(t, <-holderErr, "the holder was killed")
		next := r.land(b, "b", "land-probe-slow")
		nextOut, nextErr := runAsync(next)
		var err error
		select {
		case err = <-nextErr:
		case <-time.After(2 * time.Minute):
			_ = syscall.Kill(-next.Process.Pid, syscall.SIGKILL)
			err = <-nextErr
		}

		t.Run("it should let the next make land proceed", func(t *testing.T) {
			assert.NoError(t, err, holderOut.String()+nextOut.String())
		})
	})

	t.Run("when the rebase conflicts", func(t *testing.T) {
		t.Parallel()
		r := newLandRig(t)
		a := r.session(t, "a", "shared.txt", "a\n")
		b := r.session(t, "b", "shared.txt", "b\n")
		r.git(t, a, "push", "-q", "github", "HEAD:main")
		before := r.remoteMain(t)
		out, err := r.land(b, "b", "land-probe-slow").CombinedOutput()

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.Error(t, err, string(out))
		})

		t.Run("it should leave the remote's main where it was", func(t *testing.T) {
			assert.Equal(t, before, r.remoteMain(t))
		})

		t.Run("it should leave the rebase in progress for the session", func(t *testing.T) {
			assert.DirExists(t, filepath.Join(b, ".git", "rebase-merge"))
		})
	})

	t.Run("when flock is not on the PATH", func(t *testing.T) {
		t.Parallel()
		r := newLandRig(t)
		before := r.remoteMain(t)
		a := r.session(t, "a", "a.txt", "a\n")
		out, err := r.land(a, "a", "land-probe-slow", "PATH="+pathWithout(t, "flock")).CombinedOutput()

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.Error(t, err, string(out))
		})

		t.Run("it should say how to install flock on either platform", func(t *testing.T) {
			assert.Contains(t, string(out), "make land needs flock (util-linux on Linux, brew install flock on macOS)")
		})

		t.Run("it should leave the remote's main where it was", func(t *testing.T) {
			assert.Equal(t, before, r.remoteMain(t))
		})
	})

	t.Run("when the land recipes are read from the Makefile", func(t *testing.T) {
		recipes := makeRecipes(t, filepath.Join(repoRoot(t), "Makefile"), "land", "land-under-lock")
		require.NotEmpty(t, recipes)
		pipe := regexp.MustCompile(`(^|[^|])\|([^|]|$)`)
		var piped []string
		for _, line := range recipes {
			if pipe.MatchString(line) {
				piped = append(piped, line)
			}
		}

		t.Run("it should pipe nothing, so the gate's exit code decides the push", func(t *testing.T) {
			assert.Empty(t, piped)
		})

		// Item 202: main's CI was red for two days and no landing said so.
		t.Run("it should report main's CI after the fetch and before the gate", func(t *testing.T) {
			var order []string
			for _, line := range recipes {
				for _, step := range []string{"git fetch", "tools/ci-status", "${LAND_GATE}"} {
					if strings.Contains(line, step) {
						order = append(order, step)
					}
				}
			}
			assert.Equal(t, []string{"git fetch", "tools/ci-status", "${LAND_GATE}"}, order)
		})
	})
}

type lockedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func runAsync(cmd *exec.Cmd) (*lockedBuffer, <-chan error) {
	out := &lockedBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		done <- err
		return out, done
	}
	go func() { done <- cmd.Wait() }()
	return out, done
}

// pathWithout is a PATH holding every program on this one except name.
func pathWithout(t *testing.T, name string) string {
	t.Helper()
	bin := t.TempDir()
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Name() == name {
				continue
			}
			link := filepath.Join(bin, e.Name())
			if _, err := os.Lstat(link); err == nil {
				continue
			}
			_ = os.Symlink(filepath.Join(dir, e.Name()), link)
		}
	}
	return bin
}

// makeRecipes is the recipe lines of the named rules, as written.
func makeRecipes(t *testing.T, makefile string, targets ...string) []string {
	t.Helper()
	f, err := os.Open(makefile)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	want := map[string]bool{}
	for _, target := range targets {
		want[target] = true
	}
	var lines []string
	in := false
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "\t") {
			if in {
				lines = append(lines, line)
			}
			continue
		}
		if head, _, ok := strings.Cut(line, ":"); ok && !strings.ContainsAny(head, "=$") {
			in = want[strings.TrimSpace(head)]
		} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
			in = false
		}
	}
	require.NoError(t, scan.Err())
	return lines
}
