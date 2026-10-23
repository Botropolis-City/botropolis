package daemon_test

import (
	"sync"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sid = "08b1d655-276f-495b-a61f-2214af35a97a"
	cwd = "/home/avesta/workspaces/github/mCedar/cinders"
	pid = 4242
)

var (
	t0     = time.Date(2026, time.September, 16, 19, 50, 0, 0, time.UTC)
	now    = t0.Add(time.Hour)
	probes = state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }}
)

func stamp(d time.Duration) string {
	return t0.Add(d).Format(time.RFC3339Nano)
}

func idleHome(t *testing.T, kind string) *helpers.Home {
	t.Helper()
	home := helpers.NewHome(t).Session(pid, sid, cwd, kind, "idle")
	home.Transcript(sid, cwd,
		helpers.UserPrompt(sid, cwd, stamp(0)),
		helpers.ModelAttachment(sid, "claude-opus-5[1m]", stamp(time.Second)),
		helpers.AssistantReply(sid, "msg_01", stamp(10*time.Second)),
		helpers.AITitle(sid, "Fix the CI queue"))
	return home
}

func newDaemon(t *testing.T, home *helpers.Home) *daemon.Daemon {
	t.Helper()
	d := daemon.New(home.Path, probes, func() time.Time { return now })
	require.NoError(t, d.Rescan())
	return d
}

func only(t *testing.T, d *daemon.Daemon) state.Session {
	t.Helper()
	sessions := d.Snapshot().Sessions
	require.Len(t, sessions, 1)
	return sessions[0]
}

func event(name claude.HookName, tool string) claude.HookEvent {
	return claude.HookEvent{Name: name, SessionID: sid, CWD: cwd, ToolName: tool}
}

func TestDaemon(t *testing.T) {
	t.Run("when created over a home with one idle session", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))

		t.Run("it should expose the session in its snapshot", func(t *testing.T) {
			assert.Equal(t, "Fix the CI queue", only(t, d).Title)
		})

		t.Run("it should derive the state from the files", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})

		t.Run("it should stamp the snapshot with the clock", func(t *testing.T) {
			assert.Equal(t, now, d.Snapshot().At)
		})
	})

	t.Run("when a PreToolUse event arrives for an idle interactive session", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		updates, cancel := d.Subscribe()
		defer cancel()
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)

		t.Run("it should flip the session to working", func(t *testing.T) {
			assert.Equal(t, state.Working, only(t, d).State)
		})

		t.Run("it should record the tool in use", func(t *testing.T) {
			assert.Equal(t, "Bash", only(t, d).Tool)
		})

		t.Run("it should move last activity to the event time", func(t *testing.T) {
			assert.Equal(t, now, only(t, d).LastActivity)
		})

		t.Run("it should notify subscribers", func(t *testing.T) {
			select {
			case update := <-updates:
				assert.Equal(t, state.Working, update.Snapshot.Sessions[0].State)
			case <-time.After(time.Second):
				t.Fatal("no update delivered")
			}
		})
	})

	t.Run("when a PreToolUse event names a file", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		edit := event(claude.HookPreToolUse, "Edit")
		edit.ToolInput.FilePath = "/p/x.go"
		d.Apply(edit, now)

		t.Run("it should record what the tool is working on", func(t *testing.T) {
			assert.Equal(t, "/p/x.go", only(t, d).Subject)
		})
	})

	t.Run("when a PreToolUse event arrives for an unattended background session", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "bg"))
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)

		t.Run("it should flip the session to unattended", func(t *testing.T) {
			assert.Equal(t, state.Unattended, only(t, d).State)
		})
	})

	t.Run("when a PostToolUse event follows a PreToolUse event", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)
		d.Apply(event(claude.HookPostToolUse, "Bash"), now.Add(time.Second))

		t.Run("it should stay working", func(t *testing.T) {
			assert.Equal(t, state.Working, only(t, d).State)
		})

		t.Run("it should clear the tool in use", func(t *testing.T) {
			assert.Empty(t, only(t, d).Tool)
		})
	})

	t.Run("when a Notification event arrives", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)
		d.Apply(claude.HookEvent{Name: claude.HookNotification, SessionID: sid, NotificationType: "permission_prompt",
			Message: "Claude needs your permission to use Bash"}, now.Add(time.Second))

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})

		t.Run("it should carry the message", func(t *testing.T) {
			assert.Equal(t, "Claude needs your permission to use Bash", only(t, d).Note)
		})
	})

	t.Run("when a Stop event arrives on a working session", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)
		d.Apply(event(claude.HookStop, ""), now.Add(time.Second))

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})
	})

	t.Run("when a SessionEnd event arrives", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(event(claude.HookSessionEnd, ""), now)

		t.Run("it should park the session", func(t *testing.T) {
			assert.Equal(t, state.Resting, only(t, d).State)
		})
	})

	t.Run("when the transcript catches up after an event", func(t *testing.T) {
		home := idleHome(t, "interactive")
		d := newDaemon(t, home)
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)
		home.AppendTranscript(sid, cwd,
			helpers.UserPrompt(sid, cwd, now.Add(time.Second).Format(time.RFC3339Nano)),
			helpers.AssistantReply(sid, "msg_02", now.Add(2*time.Second).Format(time.RFC3339Nano)))
		require.NoError(t, d.Rescan())

		t.Run("it should trust the files again", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})

		t.Run("it should drop the stale tool", func(t *testing.T) {
			assert.Empty(t, only(t, d).Tool)
		})
	})

	t.Run("when the transcript is re-read but is still older than the event", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)
		require.NoError(t, d.Rescan())

		t.Run("it should keep the event's state", func(t *testing.T) {
			assert.Equal(t, state.Working, only(t, d).State)
		})
	})

	t.Run("when an event arrives for a session the files do not know", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: "ffffffff-0000-0000-0000-000000000000", ToolName: "Bash"}, now)

		t.Run("it should not invent a session", func(t *testing.T) {
			assert.Len(t, d.Snapshot().Sessions, 1)
		})
	})

	t.Run("when a Stop event follows a PreToolUse after the first scan", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)
		updates, cancel := d.Subscribe()
		defer cancel()
		d.Apply(event(claude.HookStop, ""), now.Add(time.Minute))

		t.Run("it should log a needs-you event", func(t *testing.T) {
			logged := d.Events(time.Time{})
			require.Len(t, logged, 1)
			assert.Equal(t, events.NeedsYou, logged[0].Kind)
			assert.Equal(t, "Fix the CI queue", logged[0].Title)
		})

		t.Run("it should give a subscriber the fresh events with the update", func(t *testing.T) {
			select {
			case update := <-updates:
				require.Len(t, update.Events, 1)
				assert.Equal(t, events.NeedsYou, update.Events[0].Kind)
			case <-time.After(time.Second):
				t.Fatal("no update delivered")
			}
		})
	})

	t.Run("when the first scan lands", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))

		t.Run("it should set the baseline without logging anything", func(t *testing.T) {
			assert.Empty(t, d.Events(time.Time{}))
		})
	})

	t.Run("when a hook event has been seen for a session", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))

		t.Run("it should be unhooked until then", func(t *testing.T) {
			assert.False(t, only(t, d).Hooked)
		})

		d.Apply(event(claude.HookPreToolUse, "Bash"), now)

		t.Run("it should mark the session hooked", func(t *testing.T) {
			assert.True(t, only(t, d).Hooked)
		})

		t.Run("and the file scan catches up", func(t *testing.T) {
			require.NoError(t, d.Rescan())

			t.Run("it should stay hooked", func(t *testing.T) {
				assert.True(t, only(t, d).Hooked)
			})
		})
	})

	t.Run("when hook events arrive in a burst", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		var mu sync.Mutex
		released := 0
		d.SetRelease(30*time.Millisecond, func() {
			mu.Lock()
			defer mu.Unlock()
			released++
		})
		for i := 0; i < 5; i++ {
			d.Apply(event(claude.HookPreToolUse, "Bash"), now)
		}
		count := func() int { mu.Lock(); defer mu.Unlock(); return released }

		t.Run("it should release memory once they settle", func(t *testing.T) {
			assert.Eventually(t, func() bool { return count() == 1 }, time.Second, 5*time.Millisecond)
		})

		t.Run("it should not keep releasing after that", func(t *testing.T) {
			time.Sleep(80 * time.Millisecond)
			assert.Equal(t, 1, count())
		})
	})

	releases := func(t *testing.T, d *daemon.Daemon, after, ceiling time.Duration) func() int {
		var mu sync.Mutex
		released := 0
		d.SetRelease(after, func() {
			mu.Lock()
			defer mu.Unlock()
			released++
		})
		d.SetReleaseCeiling(ceiling)
		return func() int { mu.Lock(); defer mu.Unlock(); return released }
	}

	t.Run("when the first load finishes, and no hook event follows", func(t *testing.T) {
		d := daemon.New(idleHome(t, "interactive").Path, probes, func() time.Time { return now })
		count := releases(t, d, 30*time.Millisecond, time.Minute)
		require.NoError(t, d.Rescan())

		t.Run("it should release memory once it settles", func(t *testing.T) {
			assert.Eventually(t, func() bool { return count() == 1 }, time.Second, 5*time.Millisecond)
		})
	})

	t.Run("when rescans arrive in a burst, closer together than the settle time", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		count := releases(t, d, 100*time.Millisecond, time.Minute)
		for i := 0; i < 10; i++ {
			require.NoError(t, d.Rescan())
			time.Sleep(10 * time.Millisecond)
		}
		during := count()

		t.Run("it should not release memory during the burst", func(t *testing.T) {
			assert.Equal(t, 0, during)
		})

		t.Run("it should release memory once they settle", func(t *testing.T) {
			assert.Eventually(t, func() bool { return count() == 1 }, time.Second, 5*time.Millisecond)
		})

		t.Run("it should not keep releasing after that", func(t *testing.T) {
			time.Sleep(250 * time.Millisecond)
			assert.Equal(t, 1, count())
		})
	})

	t.Run("when rescans arrive steadily for longer than the ceiling", func(t *testing.T) {
		const (
			settle  = 150 * time.Millisecond
			ceiling = 300 * time.Millisecond
			stream  = time.Second
		)
		d := newDaemon(t, idleHome(t, "interactive"))
		count := releases(t, d, settle, ceiling)
		for start := time.Now(); time.Since(start) < stream; time.Sleep(10 * time.Millisecond) {
			require.NoError(t, d.Rescan())
		}
		during := count()

		t.Run("it should release while they continue", func(t *testing.T) {
			assert.GreaterOrEqual(t, during, 1)
		})

		t.Run("it should release no more than once per ceiling", func(t *testing.T) {
			assert.LessOrEqual(t, during, int(stream/ceiling))
		})
	})

	t.Run("when a subscriber cancels", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		updates, cancel := d.Subscribe()
		cancel()
		d.Apply(event(claude.HookPreToolUse, "Bash"), now)

		t.Run("it should close the channel", func(t *testing.T) {
			_, open := <-updates
			assert.False(t, open)
		})
	})
}
