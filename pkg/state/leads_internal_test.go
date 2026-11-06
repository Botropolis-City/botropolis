package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 343: what detection needs to keep a teammate with its lead.
func TestTeamLeads(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	inTeam := func(id, agent string, started time.Duration) Session {
		return Session{ID: id, CWD: "/w/" + id, Team: "crew", Agent: agent, State: Working, Alive: true, StartedAt: now.Add(started)}
	}
	leadOf := func(t *testing.T, sessions []Session, teams []claude.Team, id string) string {
		t.Helper()
		resolveLeads(sessions, teams)
		for _, m := range members(nil, sessions, now) {
			if m.ID == id {
				return m.Lead
			}
		}
		require.Fail(t, "no member "+id)
		return ""
	}

	t.Run("when the roster names the team's lead", func(t *testing.T) {
		sessions := []Session{inTeam("l", "", 0), inTeam("w1", "worker", time.Minute), inTeam("w2", "worker", 2*time.Minute)}
		teams := []claude.Team{{Name: "crew", LeadSessionID: "l"}}

		t.Run("its teammates' members should name it as their lead", func(t *testing.T) {
			assert.Equal(t, []string{"l", "l"}, []string{leadOf(t, sessions, teams, "w1"), leadOf(t, sessions, teams, "w2")})
		})

		t.Run("its own member should name no lead", func(t *testing.T) {
			assert.Equal(t, "", leadOf(t, sessions, teams, "l"))
		})
	})

	t.Run("when the roster names a lead that is not among the sessions", func(t *testing.T) {
		sessions := []Session{inTeam("w1", "worker", time.Minute), inTeam("w2", "worker", 2*time.Minute)}
		teams := []claude.Team{{Name: "crew", LeadSessionID: "gone"}}

		t.Run("its teammates should still name it, so they follow where it is recorded", func(t *testing.T) {
			assert.Equal(t, "gone", leadOf(t, sessions, teams, "w1"))
		})
	})

	t.Run("when no roster names the team", func(t *testing.T) {
		sessions := []Session{inTeam("w1", "worker", time.Minute), inTeam("l", "", 0), inTeam("w2", "worker", 2*time.Minute)}

		t.Run("the team's session with no agent should lead, as the camp has it", func(t *testing.T) {
			assert.Equal(t, "l", leadOf(t, sessions, nil, "w1"))
		})

		t.Run("and two sessions have no agent", func(t *testing.T) {
			sessions := []Session{inTeam("w1", "worker", time.Minute), inTeam("later", "", 3*time.Minute), inTeam("l", "", 0)}

			t.Run("the first started should lead", func(t *testing.T) {
				assert.Equal(t, "l", leadOf(t, sessions, nil, "w1"))
			})

			t.Run("the other should follow it", func(t *testing.T) {
				assert.Equal(t, "l", leadOf(t, sessions, nil, "later"))
			})
		})

		t.Run("and every session has an agent", func(t *testing.T) {
			sessions := []Session{inTeam("w1", "worker", time.Minute), inTeam("w2", "worker", 2*time.Minute)}

			t.Run("none should name a lead", func(t *testing.T) {
				assert.Equal(t, "", leadOf(t, sessions, nil, "w1"))
			})
		})
	})

	t.Run("when a session is in no team", func(t *testing.T) {
		sessions := []Session{{ID: "x", CWD: "/w/x", State: Working, StartedAt: now}, inTeam("l", "", 0), inTeam("w1", "worker", time.Minute)}

		t.Run("it should name no lead", func(t *testing.T) {
			assert.Equal(t, "", leadOf(t, sessions, nil, "x"))
		})
	})

	t.Run("when a session's lead changes between two loads", func(t *testing.T) {
		before := []Session{inTeam("l", "", 0), inTeam("w1", "worker", time.Minute)}
		after := []Session{inTeam("l", "", 0), inTeam("w1", "worker", time.Minute)}
		resolveLeads(before, nil)
		resolveLeads(after, []claude.Team{{Name: "crew", LeadSessionID: "other"}})

		t.Run("the grouping signature should change, so the sessions are grouped again", func(t *testing.T) {
			assert.NotEqual(t, groupSignature(before), groupSignature(after))
		})

		t.Run("the kept parts' signature should change with it", func(t *testing.T) {
			var parts signatureParts
			assert.NotEqual(t, parts.signature(before), parts.signature(after))
		})
	})
}
