package commands_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

// Item 197: a feed for a city nobody can see lets its subscription go
// and glances at the daemon every so often, for the window's title,
// and attaches at once when the city is shown; it hands on every event
// once.
func TestFeedWhileHidden(t *testing.T) {
	// fed runs a feed on a daemon with one session, glancing every
	// every while hidden, and counts what it is handed.
	type tally struct {
		mu        sync.Mutex
		snapshots int
		events    []events.Event
	}
	fed := func(t *testing.T, every time.Duration) (*tally, *commands.Glance, func(claude.HookEvent, time.Time)) {
		t.Helper()
		d, sock := servedDaemon(t)
		glance := commands.NewGlance(every)
		got := &tally{}
		feed := commands.Feed{Socket: sock, Source: &fakeSource{}, Poll: time.Millisecond, Retry: 200 * time.Millisecond,
			Since:  func() time.Time { return daemonT0 },
			Glance: glance,
			Events: func(fresh []events.Event) {
				got.mu.Lock()
				defer got.mu.Unlock()
				got.events = append(got.events, fresh...)
			}}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go feed.Run(ctx, func(state.Snapshot) {
			got.mu.Lock()
			defer got.mu.Unlock()
			got.snapshots++
		})
		require.Eventually(t, func() bool { got.mu.Lock(); defer got.mu.Unlock(); return got.snapshots > 0 }, 2*time.Second, 5*time.Millisecond)
		return got, glance, d.Apply
	}
	count := func(got *tally) (int, int) {
		got.mu.Lock()
		defer got.mu.Unlock()
		return got.snapshots, len(got.events)
	}
	// working is the session starting a tool and stopping: one event
	// for the city's log.
	working := func(apply func(claude.HookEvent, time.Time), at time.Time) {
		apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Grep"}, at)
		apply(claude.HookEvent{Name: claude.HookStop, SessionID: sid}, at.Add(time.Second))
	}

	t.Run("when the city is hidden, and the daemon changes between glances", func(t *testing.T) {
		got, glance, apply := fed(t, time.Hour)
		glance.Hide(true)
		time.Sleep(100 * time.Millisecond)
		before, _ := count(got)
		working(apply, daemonNow.Add(time.Minute))
		time.Sleep(300 * time.Millisecond)
		during, _ := count(got)

		t.Run("it should be handed nothing until the next glance", func(t *testing.T) {
			assert.Equal(t, before, during)
		})

		t.Run("and the city is shown", func(t *testing.T) {
			glance.Hide(false)

			t.Run("it should be handed the newest snapshot at once", func(t *testing.T) {
				assert.Eventually(t, func() bool { n, _ := count(got); return n > during }, time.Second, 5*time.Millisecond)
			})

			t.Run("it should be handed the event it missed, once", func(t *testing.T) {
				assert.Eventually(t, func() bool { _, n := count(got); return n == 1 }, time.Second, 5*time.Millisecond)
			})
		})
	})

	t.Run("when the city is hidden, and glances every 50 ms", func(t *testing.T) {
		got, glance, apply := fed(t, 50*time.Millisecond)
		glance.Hide(true)
		before, _ := count(got)
		working(apply, daemonNow.Add(time.Minute))
		time.Sleep(500 * time.Millisecond)
		after, handed := count(got)

		t.Run("it should be handed a snapshot at each glance", func(t *testing.T) {
			assert.GreaterOrEqual(t, after-before, 3)
		})

		t.Run("it should be handed each event once, however many glances see it", func(t *testing.T) {
			assert.Equal(t, 1, handed)
		})
	})
}
