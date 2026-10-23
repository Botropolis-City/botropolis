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

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/testing/helpers"
)

// busyCity is item 197's city: a daemon of 200 sessions in 150
// projects, 25 of them live, one of which writes a line on demand, with
// roads between its projects as the real history has (item 206).
type busyCity struct {
	root string
	home *helpers.Home
	sock string
	at   time.Time
	line int
	// roads is how many roads its sessions' edits into each other's
	// projects should make.
	roads int
	// state is the state directory the daemon and a city share when
	// they run on the home as a person's own, keeping the statistics
	// store, or "" when they run with --home and keep none (item 215).
	state string
}

// A busy city's history is the real one's size: about 31,600 replies
// in all and 4,200 in the last day (item 215).
const (
	historyReplies = 150
	historyDays    = 7
)

func newBusyCity(t *testing.T) *busyCity {
	t.Helper()
	return newBusyCityOf(t, false)
}

// newBusyCityWithStore is the busy city run as a person's own, so its
// daemon keeps the statistics store and a city reads it, with 150
// replies behind each session spread over the last 7 days. A city that
// opens no store never pays for reading it, which is how item 215's
// 3.6% of a core went unmeasured. It returns once the daemon has stored
// the whole history.
func newBusyCityWithStore(t *testing.T) *busyCity {
	t.Helper()
	return newBusyCityOf(t, true)
}

func newBusyCityOf(t *testing.T, store bool) *busyCity {
	t.Helper()
	const (
		sessions = 200
		projects = 150
		live     = 25
		roads    = 80
	)
	daemon, err := botropolisd()
	require.NoError(t, err)
	b := &busyCity{root: t.TempDir(), home: helpers.NewHome(t), at: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	// Each of 80 sessions edits a file in another project: the 80 pairs,
	// none repeated, that realSized in pkg/city joins.
	edits := map[int]int{}
	for r := range roads {
		edits[(r*7)%projects] = (r*13 + 5) % projects
	}
	b.roads = roads
	for i := range sessions {
		sid := fmt.Sprintf("%08d-0000-4000-8000-000000000197", i)
		cwd := fmt.Sprintf("/home/user/src/project-%03d", i%projects)
		ts := b.at.Add(-time.Duration(i) * time.Minute).Format("2006-01-02T15:04:05.000Z")
		lines := []string{helpers.UserPrompt(sid, cwd, ts), helpers.AssistantReply(sid, fmt.Sprintf("msg_%06d", i), ts)}
		if to, ok := edits[i]; ok && i < projects {
			lines = append(lines, helpers.AssistantFileEdit(sid, fmt.Sprintf("msg_edit_%06d", i), ts, fmt.Sprintf("toolu_%06d", i),
				fmt.Sprintf("/home/user/src/project-%03d/main.go", to)))
		}
		if store {
			lines = append(lines, history(sid, i, sessions)...)
		}
		b.home.Transcript(sid, cwd, lines...)
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
	if store {
		b.state = filepath.Join(b.root, "state")
		d = exec.Command(daemon, "--socket", b.sock, "--config", b.config())
		d.Env = append(os.Environ(), "HOME="+b.home.Path, "XDG_STATE_HOME="+b.state, "PATH=/nonexistent")
	}
	require.NoError(t, d.Start())
	t.Cleanup(func() { _ = d.Process.Kill(); _ = d.Wait() })
	require.Eventually(t, func() bool { _, err := os.Stat(b.sock); return err == nil }, 10*time.Second, 20*time.Millisecond)
	if store {
		want := sessions * historyReplies
		require.Eventually(t, func() bool { return b.stored(t) >= want }, 5*time.Minute, time.Second,
			"the daemon never stored the %d replies of history", want)
	}
	return b
}

// history is session i's 150 replies, the sessions' together falling
// one after another over the last 7 days, newest first.
func history(sid string, i, sessions int) []string {
	step := historyDays * 24 * time.Hour / time.Duration(sessions*historyReplies)
	now := time.Now().UTC()
	lines := make([]string, 0, historyReplies)
	for j := historyReplies - 1; j >= 0; j-- {
		at := now.Add(-time.Duration(i*historyReplies+j+1) * step).Format("2006-01-02T15:04:05.000Z")
		lines = append(lines, helpers.AssistantReply(sid, fmt.Sprintf("msg_history_%03d_%03d", i, j), at))
	}
	return lines
}

// stored is how many messages the daemon's statistics store holds.
func (b *busyCity) stored(t *testing.T) int {
	t.Helper()
	s, err := stats.OpenReader(filepath.Join(b.state, "botropolis", "stats.db"))
	if err != nil {
		return 0
	}
	defer func() { _ = s.Close() }()
	var n int
	if err := s.DB().QueryRow("SELECT count(*) FROM messages").Scan(&n); err != nil {
		return 0
	}
	return n
}

// cityArgs and cityEnv are how a city is run against the busy city: on the same home
// and state directory as the daemon when it keeps the store, and with
// --home and its own state otherwise.
func (b *busyCity) cityArgs() []string {
	if b.state != "" {
		return []string{"city", "--socket", b.sock, "--config", b.config()}
	}
	return []string{"city", "--home", b.home.Path, "--socket", b.sock, "--config", b.config()}
}

func (b *busyCity) cityEnv() []string {
	if b.state != "" {
		return []string{"HOME=" + b.home.Path, "XDG_STATE_HOME=" + b.state}
	}
	return []string{"HOME=" + filepath.Join(b.root, "home"), "XDG_STATE_HOME=" + filepath.Join(b.root, "city-state")}
}

// roadsSeen is how many roads the daemon's snapshot holds, once it has
// as many as the city should have or a minute has passed.
func (b *busyCity) roadsSeen(t *testing.T) int {
	t.Helper()
	seen := 0
	for deadline := time.Now().Add(time.Minute); seen < b.roads && time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		client, err := proto.Dial(b.sock)
		if err != nil {
			continue
		}
		if snap, err := client.Snapshot(); err == nil {
			seen = len(snap.Roads)
		}
		_ = client.Close()
	}
	return seen
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
