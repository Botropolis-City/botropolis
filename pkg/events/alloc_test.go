package events_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

// Item 213: Observe made a map of every session on every rescan, a
// twentieth of what the daemon allocated among 2,000 sessions. It keeps
// two and swaps them.
func TestObserveAllocations(t *testing.T) {
	many := func(n int) []state.Session {
		sessions := make([]state.Session, n)
		for i := range sessions {
			sessions[i] = session(fmt.Sprintf("00000000-0000-4213-a000-%012d", i), cinders, state.Resting)
		}
		return sessions
	}

	t.Run("when a snapshot of 2,000 sessions is observed again with nothing new", func(t *testing.T) {
		log := events.NewLog()
		s := snapshot(many(2000)...)
		log.Observe(s, now)
		log.Observe(s, now)

		t.Run("it should allocate no map of its sessions", func(t *testing.T) {
			assert.LessOrEqual(t, testing.AllocsPerRun(10, func() { log.Observe(s, now) }), 1.0)
		})
	})

	t.Run("when a session ends between two observations", func(t *testing.T) {
		log := events.NewLog()
		sessions := many(3)
		sessions[1].State = state.Working
		log.Observe(snapshot(sessions...), now)
		log.Observe(snapshot(sessions...), now.Add(time.Second))
		ended := log.Observe(snapshot(sessions[0], sessions[2]), now.Add(2*time.Second))
		again := log.Observe(snapshot(sessions[0], sessions[2]), now.Add(3*time.Second))

		t.Run("it should log it ended", func(t *testing.T) {
			assert.Equal(t, []events.Kind{events.Ended}, kinds(ended))
		})

		t.Run("it should not log it again on the next", func(t *testing.T) {
			assert.Empty(t, again)
		})
	})
}
