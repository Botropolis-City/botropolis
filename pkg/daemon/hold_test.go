package daemon_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// workingDaemon is a daemon over one interactive session that a
// PreToolUse hook has put to work at now, on a clock the test moves.
func workingDaemon(t *testing.T) (*daemon.Daemon, *helpers.FakeClock, *helpers.Home) {
	t.Helper()
	home := idleHome(t, "interactive")
	d := newDaemon(t, home)
	clock := helpers.NewFakeClock(now)
	d.SetReleaseClock(clock)
	d.Apply(event(claude.HookPreToolUse, "Bash"), now)
	return d, clock, home
}

func needsYouEvents(d *daemon.Daemon) []events.Event {
	var found []events.Event
	for _, e := range d.Events(time.Time{}) {
		if e.Kind == events.NeedsYou {
			found = append(found, e)
		}
	}
	return found
}

// shownStates appends every state a subscriber has been handed so far,
// without waiting for more.
func shownStates(updates <-chan proto.Update, seen *[]state.State) {
	for {
		select {
		case update := <-updates:
			*seen = append(*seen, update.Snapshot.Sessions[0].State)
		default:
			return
		}
	}
}

// Item 346: a session that pauses for a second or two between turns
// keeps showing as working, and needs you only once the hold passes
// with nothing new.
func TestIdleHold(t *testing.T) {
	t.Run("when a working session's turn ends", func(t *testing.T) {
		d, clock, _ := workingDaemon(t)
		updates, cancel := d.Subscribe()
		defer cancel()
		d.Apply(event(claude.HookStop, ""), now.Add(time.Second))
		clock.Advance(daemon.IdleHold - time.Millisecond)

		t.Run("it should still show working inside the hold", func(t *testing.T) {
			assert.Equal(t, state.Working, only(t, d).State)
		})

		t.Run("it should keep showing the tool it was using", func(t *testing.T) {
			assert.Equal(t, "Bash", only(t, d).Tool)
		})

		t.Run("it should log no needs-you event inside the hold", func(t *testing.T) {
			assert.Empty(t, needsYouEvents(d))
		})

		t.Run("and the hold passes with nothing new", func(t *testing.T) {
			clock.Advance(time.Millisecond)

			t.Run("it should need you", func(t *testing.T) {
				assert.Equal(t, state.NeedsYou, only(t, d).State)
			})

			t.Run("it should drop the tool", func(t *testing.T) {
				assert.Empty(t, only(t, d).Tool)
			})

			t.Run("it should log a needs-you event", func(t *testing.T) {
				assert.Len(t, needsYouEvents(d), 1)
			})

			t.Run("it should notify subscribers", func(t *testing.T) {
				var seen []state.State
				shownStates(updates, &seen)
				require.NotEmpty(t, seen)
				assert.Equal(t, state.NeedsYou, seen[len(seen)-1])
			})
		})
	})

	t.Run("when a working session's turn ends and it works again inside the hold", func(t *testing.T) {
		d, clock, _ := workingDaemon(t)
		updates, cancel := d.Subscribe()
		defer cancel()
		var seen []state.State
		d.Apply(event(claude.HookStop, ""), now.Add(time.Second))
		shownStates(updates, &seen)
		clock.Advance(time.Second)
		shownStates(updates, &seen)
		d.Apply(event(claude.HookPreToolUse, "Edit"), now.Add(2*time.Second))
		shownStates(updates, &seen)
		clock.Advance(2 * daemon.IdleHold)
		shownStates(updates, &seen)

		t.Run("it should show working", func(t *testing.T) {
			assert.Equal(t, state.Working, only(t, d).State)
		})

		t.Run("it should show the new tool", func(t *testing.T) {
			assert.Equal(t, "Edit", only(t, d).Tool)
		})

		t.Run("it should never have shown needing you", func(t *testing.T) {
			assert.NotContains(t, seen, state.NeedsYou)
		})

		t.Run("it should log no needs-you event", func(t *testing.T) {
			assert.Empty(t, needsYouEvents(d))
		})
	})

	t.Run("when a working session's record turns idle on a rescan", func(t *testing.T) {
		d, clock, home := workingDaemon(t)
		home.AppendTranscript(sid, cwd,
			helpers.UserPrompt(sid, cwd, now.Add(time.Second).Format(time.RFC3339Nano)),
			helpers.AssistantReply(sid, "msg_02", now.Add(2*time.Second).Format(time.RFC3339Nano)))
		require.NoError(t, d.Rescan())

		t.Run("it should still show working inside the hold", func(t *testing.T) {
			assert.Equal(t, state.Working, only(t, d).State)
		})

		t.Run("and the hold passes", func(t *testing.T) {
			clock.Advance(daemon.IdleHold)

			t.Run("it should need you", func(t *testing.T) {
				assert.Equal(t, state.NeedsYou, only(t, d).State)
			})
		})
	})

	t.Run("when a working session hits a permission prompt", func(t *testing.T) {
		d, _, _ := workingDaemon(t)
		d.Apply(claude.HookEvent{Name: claude.HookNotification, SessionID: sid, NotificationType: "permission_prompt",
			Message: "Claude needs your permission to use Bash"}, now.Add(time.Second))

		t.Run("it should need you at once", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})
	})

	t.Run("when a working session's record turns to waiting on a permission prompt", func(t *testing.T) {
		d, _, home := workingDaemon(t)
		home.WaitingSession(pid, sid, cwd, "interactive", "permission prompt")
		home.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, now.Add(time.Second).Format(time.RFC3339Nano)))
		require.NoError(t, d.Rescan())

		t.Run("it should need you at once", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})
	})

	t.Run("when a permission prompt follows a turn's end inside the hold", func(t *testing.T) {
		d, _, _ := workingDaemon(t)
		d.Apply(event(claude.HookStop, ""), now.Add(time.Second))
		d.Apply(claude.HookEvent{Name: claude.HookNotification, SessionID: sid, NotificationType: "permission_prompt",
			Message: "Claude needs your permission to use Bash"}, now.Add(2*time.Second))

		t.Run("it should need you at once", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})
	})

	t.Run("when a working session ends", func(t *testing.T) {
		d, _, _ := workingDaemon(t)
		d.Apply(event(claude.HookSessionEnd, ""), now.Add(time.Second))

		t.Run("it should rest at once", func(t *testing.T) {
			assert.Equal(t, state.Resting, only(t, d).State)
		})
	})

	t.Run("when a session that was never working needs you", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.SetReleaseClock(helpers.NewFakeClock(now))
		d.Apply(event(claude.HookStop, ""), now)

		t.Run("it should need you at once", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, only(t, d).State)
		})
	})
}
