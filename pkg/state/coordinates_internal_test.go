package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/group"
)

// Item 90: what detection needs to see a coordinating session.
func TestCoordinatingMembers(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	live := func(id, title string) Session {
		return Session{ID: id, CWD: "/w/" + id, Title: title, State: Working, Alive: true, StartedAt: now}
	}
	memberOf := func(t *testing.T, sessions []Session, id string) []string {
		t.Helper()
		resolveMessaged(sessions)
		for _, m := range members(nil, sessions, now) {
			if m.ID == id {
				return m.Messaged
			}
		}
		require.Fail(t, "no member "+id)
		return nil
	}

	t.Run("when a session's Visits are loaded", func(t *testing.T) {
		f := live("f", "foreman")
		f.Visits = map[string]int{"/w/lib": 2}
		f.VisitSpans = map[string]Span{"/w/lib": {From: now.Add(-time.Hour), To: now}}
		ms := members(nil, []Session{f}, now)

		t.Run("its member should carry them, with when the subagents were there", func(t *testing.T) {
			assert.Equal(t, []group.Visit{{Dir: "/w/lib", From: now.Add(-time.Hour), To: now}}, ms[0].Visited)
		})
	})

	t.Run("when a session messaged another by its name", func(t *testing.T) {
		f := live("f", "foreman")
		f.Messages = map[string]int{"item 90": 3}

		t.Run("its member's Messaged should hold that session's id", func(t *testing.T) {
			assert.Equal(t, []string{"w"}, memberOf(t, []Session{f, live("w", "item 90")}, "f"))
		})
	})

	t.Run("when the name it messaged matches two live sessions", func(t *testing.T) {
		f := live("f", "foreman")
		f.Messages = map[string]int{"item 90": 3}

		t.Run("it should record nothing, the name being ambiguous", func(t *testing.T) {
			assert.Empty(t, memberOf(t, []Session{f, live("w", "item 90"), live("x", "item 90")}, "f"))
		})
	})
}
