//go:build linux

package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// busyCity is item 197's city: a daemon of 200 sessions in 150
// projects, 25 of them live, one of which writes a line on demand.
type busyCity struct {
	root string
	home *helpers.Home
	sock string
	at   time.Time
	line int
}

func newBusyCity(t *testing.T) *busyCity {
	t.Helper()
	const (
		sessions = 200
		projects = 150
		live     = 25
	)
	daemon, err := botropolisd()
	require.NoError(t, err)
	b := &busyCity{root: t.TempDir(), home: helpers.NewHome(t), at: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	for i := range sessions {
		sid := fmt.Sprintf("%08d-0000-4000-8000-000000000197", i)
		cwd := fmt.Sprintf("/home/user/src/project-%03d", i%projects)
		ts := b.at.Add(-time.Duration(i) * time.Minute).Format("2006-01-02T15:04:05.000Z")
		b.home.Transcript(sid, cwd, helpers.UserPrompt(sid, cwd, ts), helpers.AssistantReply(sid, fmt.Sprintf("msg_%06d", i), ts))
		if i < live {
			// A live session has a record and a process, and only a live
			// session's transcript is watched.
			child := exec.Command("sleep", "600")
			require.NoError(t, child.Start())
			t.Cleanup(func() { _ = child.Process.Kill(); _, _ = child.Process.Wait() })
			b.home.Session(child.Process.Pid, sid, cwd, "interactive", "busy")
		}
	}
	b.sock = shortSocket(t)
	d := exec.Command(daemon, "--home", b.home.Path, "--socket", b.sock, "--config", b.config())
	d.Env = append(os.Environ(), "XDG_STATE_HOME="+filepath.Join(b.root, "daemon-state"), "PATH=/nonexistent")
	require.NoError(t, d.Start())
	t.Cleanup(func() { _ = d.Process.Kill(); _ = d.Wait() })
	require.Eventually(t, func() bool { _, err := os.Stat(b.sock); return err == nil }, 10*time.Second, 20*time.Millisecond)
	return b
}

func (b *busyCity) config() string {
	return filepath.Join(b.root, "config.toml")
}

// write has the first session write a line every so often for a while.
func (b *busyCity) write(every, over time.Duration) time.Duration {
	sid, cwd := fmt.Sprintf("%08d-0000-4000-8000-000000000197", 0), "/home/user/src/project-000"
	start := time.Now()
	for time.Since(start) < over {
		b.home.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, b.at.Add(time.Duration(b.line)*time.Second).Format(time.RFC3339Nano)))
		b.line++
		time.Sleep(every)
	}
	return time.Since(start)
}
