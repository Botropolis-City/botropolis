package state_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

func TestQuiet(t *testing.T) {
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour
	loose := func(edit func(*state.Session)) []state.Session {
		s := state.Session{ID: "s", State: state.Resting, LastActivity: now.Add(-week)}
		edit(&s)
		return []state.Session{s}
	}
	quiet := func(sessions []state.Session) bool {
		return len(state.Quiet(sessions, now, week)) == 1
	}

	t.Run("when a session is resting, in no group, with its process closed", func(t *testing.T) {
		cases := []struct {
			name string
			edit func(*state.Session)
			want bool
		}{
			{"and it was last active exactly quiet_days ago", func(*state.Session) {}, true},
			{"and it was last active long before quiet_days", func(s *state.Session) { s.LastActivity = now.Add(-5 * week) }, true},
			{"and it was last active a minute short of quiet_days", func(s *state.Session) { s.LastActivity = now.Add(-week + time.Minute) }, false},
			{"and its last activity is unknown", func(s *state.Session) { s.LastActivity = time.Time{} }, false},
			{"and its last activity is ahead of the clock", func(s *state.Session) { s.LastActivity = now.Add(time.Hour) }, false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				t.Run("it should be quiet only when it has rested long enough", func(t *testing.T) {
					assert.Equal(t, c.want, quiet(loose(c.edit)))
				})
			})
		}
	})

	t.Run("when an old resting session is in a group", func(t *testing.T) {
		for _, likely := range []bool{false, true} {
			t.Run("and its tie is likely="+boolWord(likely), func(t *testing.T) {
				sessions := loose(func(s *state.Session) { s.Group, s.GroupLikely = "g1", likely })

				t.Run("it should not be quiet", func(t *testing.T) {
					assert.False(t, quiet(sessions))
				})
			})
		}
	})

	t.Run("when an old loose session is not resting", func(t *testing.T) {
		for _, st := range []state.State{state.NeedsYou, state.Working, state.Waiting, state.Unattended, state.Empty} {
			t.Run("and it is "+string(st), func(t *testing.T) {
				sessions := loose(func(s *state.Session) { s.State = st })

				t.Run("it should not be quiet", func(t *testing.T) {
					assert.False(t, quiet(sessions))
				})
			})
		}
	})

	t.Run("when an old loose resting session still has its process open", func(t *testing.T) {
		sessions := loose(func(s *state.Session) { s.Alive = true })

		t.Run("it should not be quiet", func(t *testing.T) {
			assert.False(t, quiet(sessions))
		})
	})

	t.Run("when an old loose resting session is already packed up", func(t *testing.T) {
		sessions := loose(func(s *state.Session) { s.Packed = true })

		t.Run("it should not be quiet", func(t *testing.T) {
			assert.False(t, quiet(sessions))
		})
	})

	t.Run("when sessions are mixed", func(t *testing.T) {
		sessions := []state.Session{
			{ID: "old", State: state.Resting, LastActivity: now.Add(-2 * week)},
			{ID: "grouped", State: state.Resting, LastActivity: now.Add(-2 * week), Group: "g"},
			{ID: "fresh", State: state.Resting, LastActivity: now.Add(-time.Hour)},
			{ID: "older", State: state.Resting, LastActivity: now.Add(-3 * week)},
		}

		t.Run("it should return the quiet ids in their order", func(t *testing.T) {
			assert.Equal(t, []string{"old", "older"}, state.Quiet(sessions, now, week))
		})
	})
}

func TestQuietFor(t *testing.T) {
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour

	t.Run("when an old resting session is in a group", func(t *testing.T) {
		s := state.Session{State: state.Resting, LastActivity: now.Add(-week), Group: "g"}

		t.Run("it should still be quiet for the daemon's re-check, which never looks at groups", func(t *testing.T) {
			assert.True(t, s.QuietFor(now, week))
		})
	})
}

func boolWord(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
