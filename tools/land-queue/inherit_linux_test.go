//go:build linux

package main

import (
	"fmt"
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
)

// Item 326. A daemon started by a locked command inherited the lock's
// descriptor and held the lock for four days after the run died.

func (r *rig) pidFrom(t *testing.T, path string) int {
	t.Helper()
	var pid int
	eventually(t, func() bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil
	}, "no pid in "+path)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid
}

// opens reports whether pid has a descriptor open on path.
func opens(t *testing.T, pid int, path string) bool {
	t.Helper()
	want, err := os.Stat(path)
	require.NoError(t, err)
	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	require.NoError(t, err)
	for _, fd := range fds {
		got, err := os.Stat(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
		if err == nil && os.SameFile(want, got) {
			return true
		}
	}
	return false
}

func gone(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true
	}
	fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
	return len(fields) > 0 && fields[0] == "Z"
}

func TestInheritance(t *testing.T) {
	t.Parallel()

	t.Run("when the locked command starts a daemon, and exits", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		pidFile := filepath.Join(r.dir, "daemon.pid")
		run := r.run(t, "starter", "sleep 300 </dev/null >/dev/null 2>&1 & echo $! > "+pidFile)
		require.NoError(t, run.wait(t), run.out.String())
		daemon := r.pidFrom(t, pidFile)

		t.Run("it should leave the daemon no descriptor to the lock", func(t *testing.T) {
			assert.False(t, opens(t, daemon, r.lock))
		})

		t.Run("it should leave the daemon no descriptor into the queue", func(t *testing.T) {
			assert.Empty(t, under(t, daemon, r.queue))
		})

		t.Run("it should let the next landing take the lock", func(t *testing.T) {
			next := r.run(t, "next", r.served("next"))
			assert.NoError(t, next.wait(t), next.out.String())
		})
	})

	t.Run("when land-queue is killed alone while its command runs", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		pidFile := filepath.Join(r.dir, "command.pid")
		run := r.run(t, "killed", "echo $$ > "+pidFile+"; "+r.holding("killed"))
		command := r.pidFrom(t, pidFile)
		require.NoError(t, run.cmd.Process.Kill())

		t.Run("it should stop the command", func(t *testing.T) {
			eventually(t, func() bool { return gone(command) }, "the command outlived land-queue")
		})
	})
}

// under lists pid's descriptors whose targets lie in dir, deleted ones too.
func under(t *testing.T, pid int, dir string) []string {
	t.Helper()
	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	require.NoError(t, err)
	var found []string
	for _, fd := range fds {
		target, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
		if err == nil && strings.HasPrefix(target, dir+"/") {
			found = append(found, target)
		}
	}
	return found
}

func TestNamingTheHolder(t *testing.T) {
	t.Parallel()
	soon := "LAND_QUEUE_NAME_HOLDER_AFTER=200ms"

	t.Run("when the lock is held outside the queue past the wait", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		pidFile := filepath.Join(r.dir, "outside.pid")
		outside := exec.Command("sh", "-c", "exec 9>"+r.lock+"; flock 9; echo $$ > "+pidFile+"; exec sleep 300")
		require.NoError(t, outside.Start())
		t.Cleanup(func() { _ = outside.Process.Kill(); _ = outside.Wait() })
		holder := r.pidFrom(t, pidFile)
		w := r.runWith(t, "waiter", r.served("waiter"), soon)
		eventually(t, func() bool { return strings.Contains(w.out.String(), "pid ") }, "the waiter never named a holder: "+w.out.String())

		t.Run("it should name the holder's pid", func(t *testing.T) {
			assert.Contains(t, w.out.String(), fmt.Sprintf("pid %d (sleep 300)", holder))
		})
	})

	t.Run("when a landing ahead in the queue holds it past the wait", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		ahead := r.run(t, "ahead", r.holding("ahead"))
		r.waitForLog(t, "ahead")
		w := r.runWith(t, "waiter", r.served("waiter"), soon)
		eventually(t, func() bool { return strings.Contains(w.out.String(), "pid ") }, "the waiter never named a holder: "+w.out.String())

		t.Run("it should name the landing's pid", func(t *testing.T) {
			assert.Contains(t, w.out.String(), fmt.Sprintf("pid %d (", ahead.cmd.Process.Pid))
		})
	})

	t.Run("when the wait is shorter than the threshold", func(t *testing.T) {
		t.Parallel()
		r := newRig(t)
		ahead := r.run(t, "ahead", r.holding("ahead"))
		r.waitForLog(t, "ahead")
		w := r.runWith(t, "waiter", r.served("waiter"), "LAND_QUEUE_NAME_HOLDER_AFTER=1h")
		r.waitForTickets(t, 2)
		time.Sleep(500 * time.Millisecond)
		r.release(t, "ahead")
		require.NoError(t, ahead.wait(t))
		require.NoError(t, w.wait(t))

		t.Run("it should not name anyone", func(t *testing.T) {
			assert.NotContains(t, w.out.String(), "pid ")
		})
	})
}
