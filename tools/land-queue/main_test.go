package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 295. flock serves its waiters in no order, so a landing could wait
// behind later arrivals for most of an hour. These run the built tool as real
// processes against a lock and a queue in a temp dir, never /tmp's.

var tool string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "land-queue-test")
	if err != nil {
		panic(err)
	}
	tool = filepath.Join(dir, "land-queue")
	if out, err := exec.Command("go", "build", "-o", tool, ".").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type rig struct {
	dir, lock, queue, log string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	dir := t.TempDir()
	return &rig{
		dir:   dir,
		lock:  filepath.Join(dir, "land.lock"),
		queue: filepath.Join(dir, "land.queue"),
		log:   filepath.Join(dir, "probe.log"),
	}
}

type proc struct {
	cmd  *exec.Cmd
	out  *buffer
	done chan error
}

type buffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// worktree is a directory standing in for a session's worktree, a git
// repository with one commit when withHead is set.
func (r *rig) worktree(t *testing.T, name string, withHead bool) string {
	t.Helper()
	dir := filepath.Join(r.dir, "wt-"+name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	if withHead {
		for _, args := range [][]string{
			{"init", "-q", "-b", "main"},
			{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", name},
		} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
		}
	}
	return dir
}

// run starts the tool in its own worktree. script runs as the queued command.
func (r *rig) run(t *testing.T, name, script string) *proc {
	t.Helper()
	cmd := exec.Command(tool, "run", "--lock", r.lock, "--queue", r.queue, "--name", "land", "--", "sh", "-c", script)
	cmd.Dir = r.worktree(t, name, false)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	p := &proc{cmd: cmd, out: &buffer{}, done: make(chan error, 1)}
	cmd.Stdout, cmd.Stderr = p.out, p.out
	require.NoError(t, cmd.Start())
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	return p
}

func (p *proc) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-p.done:
		return err
	case <-time.After(time.Minute):
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		t.Fatalf("still running after a minute: %s", p.out.String())
		return nil
	}
}

func (p *proc) kill(t *testing.T) {
	t.Helper()
	require.NoError(t, syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL))
	require.Error(t, <-p.done, "it was killed")
}

// served is a command that logs its name.
func (r *rig) served(name string) string {
	return fmt.Sprintf("echo %s >> %s", name, r.log)
}

// holding is a command that logs its start, then holds until released.
func (r *rig) holding(name string) string {
	release := filepath.Join(r.dir, "release-"+name)
	return fmt.Sprintf("echo %s >> %s; while [ ! -e %s ]; do sleep 0.05; done", name, r.log, release)
}

func (r *rig) release(t *testing.T, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, "release-"+name), nil, 0o644))
}

func (r *rig) logged(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(r.log)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return strings.Fields(string(data))
}

func (r *rig) waitForLog(t *testing.T, name string) {
	t.Helper()
	eventually(t, func() bool {
		for _, line := range r.logged(t) {
			if line == name {
				return true
			}
		}
		return false
	}, name+" never started")
}

func (r *rig) tickets(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(r.queue)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if strings.Trim(e.Name(), "0123456789") == "" {
			names = append(names, e.Name())
		}
	}
	return names
}

func (r *rig) waitForTickets(t *testing.T, n int) {
	t.Helper()
	eventually(t, func() bool { return len(r.tickets(t)) == n }, fmt.Sprintf("the queue never held %d tickets", n))
}

func eventually(t *testing.T, ok func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}

func (r *rig) list(t *testing.T) string {
	t.Helper()
	out, err := exec.Command(tool, "list", "--lock", r.lock, "--queue", r.queue).CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

// queueBehindHolder starts a holder and then each waiter, one after another,
// each only once the one before it holds a ticket.
func (r *rig) queueBehindHolder(t *testing.T, waiters ...string) (*proc, []*proc) {
	t.Helper()
	holder := r.run(t, "holder", r.holding("holder"))
	r.waitForLog(t, "holder")
	var ps []*proc
	for i, name := range waiters {
		ps = append(ps, r.run(t, name, r.served(name)))
		r.waitForTickets(t, i+2)
	}
	return holder, ps
}

func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("when a holder has the lock, and three waiters arrive in order", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		_, waiters := r.queueBehindHolder(t, "first", "second", "third")
		r.release(t, "holder")
		for _, w := range waiters {
			require.NoError(t, w.wait(t), w.out.String())
		}

		t.Run("it should serve the waiters in arrival order", func(t *testing.T) {
			assert.Equal(t, []string{"holder", "first", "second", "third"}, r.logged(t))
		})

		t.Run("it should leave the queue empty", func(t *testing.T) {
			assert.Empty(t, r.tickets(t))
		})
	})

	// Waiters spaced apart on a quiet machine tend to be woken in order, so
	// the case above can pass on a bare flock. Under load the waiter next in
	// line is slow to wake and a later arrival takes the lock first; a stopped
	// waiter makes that slowness certain.
	t.Run("when the waiter next in line is slow to wake, and a later one arrives as the lock frees", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		holder, waiters := r.queueBehindHolder(t, "earlier")
		require.NoError(t, syscall.Kill(-waiters[0].cmd.Process.Pid, syscall.SIGSTOP))
		r.release(t, "holder")
		require.NoError(t, holder.wait(t), holder.out.String())
		later := r.run(t, "later", r.served("later"))
		eventually(t, func() bool {
			return len(r.tickets(t)) == 2 || strings.Contains(strings.Join(r.logged(t), " "), "later")
		}, "the later waiter neither queued nor ran")
		time.Sleep(500 * time.Millisecond)
		require.NoError(t, syscall.Kill(-waiters[0].cmd.Process.Pid, syscall.SIGCONT))
		require.NoError(t, waiters[0].wait(t), waiters[0].out.String())
		require.NoError(t, later.wait(t), later.out.String())

		t.Run("it should still serve the earlier one first", func(t *testing.T) {
			assert.Equal(t, []string{"holder", "earlier", "later"}, r.logged(t))
		})
	})

	t.Run("when a queued waiter is killed", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		_, waiters := r.queueBehindHolder(t, "first", "killed", "third")
		waiters[1].kill(t)
		r.release(t, "holder")
		require.NoError(t, waiters[0].wait(t), waiters[0].out.String())
		require.NoError(t, waiters[2].wait(t), waiters[2].out.String())

		t.Run("it should serve the waiters behind it", func(t *testing.T) {
			assert.Equal(t, []string{"holder", "first", "third"}, r.logged(t))
		})

		t.Run("it should drop its ticket from the queue", func(t *testing.T) {
			assert.Empty(t, r.tickets(t))
		})
	})

	t.Run("when the holder is killed while holding", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		holder, waiters := r.queueBehindHolder(t, "next")
		holder.kill(t)
		err := waiters[0].wait(t)

		t.Run("it should release to the next waiter", func(t *testing.T) {
			assert.NoError(t, err, waiters[0].out.String())
		})
	})

	t.Run("when the lock is held outside the queue", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		f, err := os.OpenFile(r.lock, os.O_CREATE|os.O_RDWR, 0o644)
		require.NoError(t, err)
		require.NoError(t, syscall.Flock(int(f.Fd()), syscall.LOCK_EX))
		w := r.run(t, "waiter", r.served("waiter"))
		r.waitForTickets(t, 1)
		time.Sleep(500 * time.Millisecond)
		before := r.logged(t)
		require.NoError(t, f.Close())
		require.NoError(t, w.wait(t), w.out.String())

		t.Run("it should wait for it to be released", func(t *testing.T) {
			assert.Empty(t, before)
		})
	})

	t.Run("when the command fails", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		err := r.run(t, "failing", "exit 7").wait(t)
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)

		t.Run("it should exit with the command's code", func(t *testing.T) {
			assert.Equal(t, 7, exit.ExitCode())
		})
	})

	t.Run("when the lock is held, and two wait", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		_, waiters := r.queueBehindHolder(t, "first", "second")
		r.release(t, "holder")
		require.NoError(t, waiters[0].wait(t))
		require.NoError(t, waiters[1].wait(t))

		t.Run("it should tell the second it is waiting, and how many are ahead", func(t *testing.T) {
			assert.Contains(t, waiters[1].out.String(), "Waiting for the landing in progress to finish ("+r.lock+"); 2 ahead in the queue")
		})
	})

	t.Run("when nothing holds the lock", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		w := r.run(t, "alone", r.served("alone"))
		require.NoError(t, w.wait(t))

		t.Run("it should not say it is waiting", func(t *testing.T) {
			assert.NotContains(t, w.out.String(), "Waiting")
		})
	})
}

func TestList(t *testing.T) {
	t.Parallel()

	t.Run("when a holder and two waiters are queued", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		holderDir := r.worktree(t, "holder", true)
		head, err := exec.Command("git", "-C", holderDir, "rev-parse", "--short", "HEAD").Output()
		require.NoError(t, err)
		r.queueBehindHolder(t, "first", "second")
		out := r.list(t)
		lines := strings.Split(strings.TrimSpace(out), "\n")

		for _, name := range []string{"holder", "first", "second"} {
			t.Run("it should name "+name+"'s worktree", func(t *testing.T) {
				assert.Contains(t, out, filepath.Join(r.dir, "wt-"+name))
			})
		}

		t.Run("it should name each one's target", func(t *testing.T) {
			assert.Equal(t, 3, strings.Count(out, " land "))
		})

		t.Run("it should mark the first in line as holding the lock", func(t *testing.T) {
			assert.Contains(t, lines[len(lines)-3], "holding")
		})

		t.Run("it should mark the others as waiting", func(t *testing.T) {
			assert.Equal(t, 2, strings.Count(out, "waiting"))
		})

		t.Run("it should name the head each worktree is landing", func(t *testing.T) {
			assert.Contains(t, lines[len(lines)-3], strings.TrimSpace(string(head)))
		})
	})

	t.Run("when a queued waiter has died", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		_, waiters := r.queueBehindHolder(t, "dead")
		waiters[0].kill(t)
		out := r.list(t)

		t.Run("it should leave it out", func(t *testing.T) {
			assert.NotContains(t, out, "wt-dead")
		})

		t.Run("it should leave its ticket for a waiter to clear", func(t *testing.T) {
			assert.Len(t, r.tickets(t), 2)
		})
	})

	t.Run("when the queue is empty", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)

		t.Run("it should say so", func(t *testing.T) {
			assert.Contains(t, r.list(t), "Nobody is landing or waiting")
		})
	})

	t.Run("when the lock is held outside the queue", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		f, err := os.OpenFile(r.lock, os.O_CREATE|os.O_RDWR, 0o644)
		require.NoError(t, err)
		require.NoError(t, syscall.Flock(int(f.Fd()), syscall.LOCK_EX))
		t.Cleanup(func() { _ = f.Close() })

		t.Run("it should say something outside the queue holds it", func(t *testing.T) {
			assert.Contains(t, r.list(t), "held outside the queue")
		})
	})
}
