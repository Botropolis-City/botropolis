//go:build linux

package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// settleDirs dates every directory under root a minute back, as a home
// that has not just been written looks: a listing taken within a tick of
// a directory's last change is taken again on the next load (item 196),
// which a home made seconds before a measure pays on every load and a
// real one does not (item 205).
func settleDirs(t *testing.T, root string) {
	t.Helper()
	at := time.Now().Add(-time.Minute)
	require.NoError(t, filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil || !e.IsDir() {
			return err
		}
		return os.Chtimes(path, at, at)
	}))
}

// ownDaemon is botropolisd started as a person's own daemon is: on a
// scratch HOME, with its own state directory and no --home, so it opens
// the statistics store and keeps the ledger. Item 205: the daemon budget
// tests passed --home, which opens neither, so for items 157, 196 and
// 198 they measured a daemon that ingested nothing.
type ownDaemon struct {
	pid     int
	state   string
	started time.Time
	trace   *syncBuffer
}

func startOwnDaemon(t *testing.T, bin, home string, env ...string) *ownDaemon {
	t.Helper()
	root := t.TempDir()
	sock := shortSocket(t)
	state := filepath.Join(root, "state")
	cmd := exec.Command(bin, "--socket", sock, "--config", filepath.Join(root, "config.toml"))
	cmd.Env = append(append(os.Environ(), "HOME="+home, "XDG_STATE_HOME="+state, "PATH=/nonexistent"), env...)
	trace := &syncBuffer{}
	cmd.Stderr = trace
	started := time.Now()
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	require.Eventually(t, func() bool { _, err := os.Stat(sock); return err == nil }, time.Minute, 20*time.Millisecond)
	return &ownDaemon{pid: cmd.Process.Pid, state: filepath.Join(state, "botropolis"), started: started, trace: trace}
}

// settle waits until the daemon has used at most a tick of CPU in two
// seconds, and fails the test if it never does within limit.
func (d *ownDaemon) settle(t *testing.T, limit time.Duration) {
	t.Helper()
	settled := false
	for last, deadline := cpuTicks(t, d.pid), time.Now().Add(limit); !settled && time.Now().Before(deadline); {
		time.Sleep(2 * time.Second)
		ticks := cpuTicks(t, d.pid)
		settled, last = ticks-last <= 1, ticks
	}
	require.True(t, settled, "the daemon never settled")
}

// writtenSince is whether the file in the daemon's state, or its
// write-ahead log, was written since at.
func (d *ownDaemon) writtenSince(name string, at time.Time) bool {
	for _, path := range []string{filepath.Join(d.state, name), filepath.Join(d.state, name+"-wal")} {
		if info, err := os.Stat(path); err == nil && !info.ModTime().Before(at) {
			return true
		}
	}
	return false
}

// assertProduction checks the daemon did what a person's own does, so
// a flag or a default cannot quietly turn a budget test back into a
// daemon with no ingest. Call it after the measured stretch.
func (d *ownDaemon) assertProduction(t *testing.T, measuredFrom time.Time) {
	t.Helper()
	t.Run("it should have written its statistics store while measured", func(t *testing.T) {
		assert.True(t, d.writtenSince("stats.db", measuredFrom))
	})

	t.Run("it should have kept its ledger", func(t *testing.T) {
		assert.True(t, d.writtenSince("sessions.json", d.started))
	})
}
