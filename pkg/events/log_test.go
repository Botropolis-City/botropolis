package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

const (
	cinders    = "/home/avesta/workspaces/github/mCedar/cinders"
	botropolis = "/home/avesta/workspaces/github/auroq/botropolis"
)

var now = time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)

func session(id, cwd string, s state.State) state.Session {
	return state.Session{ID: id, CWD: cwd, State: s, Title: "Fix the CI queue"}
}

func snapshot(sessions ...state.Session) state.Snapshot {
	return state.Snapshot{At: now, Sessions: sessions}
}

func kinds(list []events.Event) []events.Kind {
	out := make([]events.Kind, 0, len(list))
	for _, e := range list {
		out = append(out, e.Kind)
	}
	return out
}

func TestEventLog(t *testing.T) {
	t0 := now
	t.Run("when the first snapshot is observed", func(t *testing.T) {
		log := events.NewLog()
		fresh := log.Observe(snapshot(session("a", cinders, state.NeedsYou)), t0)

		t.Run("it should take note without reporting anything", func(t *testing.T) {
			assert.Empty(t, fresh)
			assert.Empty(t, log.Events())
		})
	})

	t.Run("when a session's turn comes back to you", func(t *testing.T) {
		log := events.NewLog()
		log.Observe(snapshot(session("a", cinders, state.Working)), t0)
		fresh := log.Observe(snapshot(session("a", cinders, state.NeedsYou)), t0.Add(time.Minute))

		t.Run("it should log a needs-you event with the title", func(t *testing.T) {
			require.Len(t, fresh, 1)
			assert.Equal(t, events.NeedsYou, fresh[0].Kind)
			assert.Equal(t, "Fix the CI queue", fresh[0].Title)
			assert.Equal(t, "a", fresh[0].SessionID)
			assert.Equal(t, t0.Add(time.Minute), fresh[0].At)
		})

		t.Run("and it stays that way", func(t *testing.T) {
			again := log.Observe(snapshot(session("a", cinders, state.NeedsYou)), t0.Add(2*time.Minute))

			t.Run("it should not log it twice", func(t *testing.T) {
				assert.Empty(t, again)
			})
		})
	})

	t.Run("when things happen to a session", func(t *testing.T) {
		before := session("a", cinders, state.Working)
		after := before
		after.APIErrors, after.LastErrorAt = 2, t0.Add(time.Minute)
		after.Compactions = 1
		after.PRs = []claude.PR{{Number: 7, Repository: "mCedar/cinders", State: claude.PROpen}}
		log := events.NewLog()
		log.Observe(snapshot(before), t0)
		fresh := log.Observe(snapshot(after), t0.Add(time.Minute))

		t.Run("it should log an error, a compaction and a PR", func(t *testing.T) {
			assert.ElementsMatch(t, []events.Kind{events.Error, events.Compaction, events.PR}, kinds(fresh))
		})

		t.Run("it should say which PR", func(t *testing.T) {
			for _, e := range fresh {
				if e.Kind == events.PR {
					assert.Equal(t, "#7 mCedar/cinders", e.Detail)
				}
			}
		})

		t.Run("and the PR merges", func(t *testing.T) {
			merged := after
			merged.PRs = []claude.PR{{Number: 7, Repository: "mCedar/cinders", State: claude.PRMerged}}
			fresh := log.Observe(snapshot(merged), t0.Add(2*time.Minute))

			t.Run("it should log the merge", func(t *testing.T) {
				assert.Equal(t, []events.Kind{events.Merged}, kinds(fresh))
			})
		})
	})

	t.Run("when sessions come and go", func(t *testing.T) {
		log := events.NewLog()
		log.Observe(snapshot(session("a", cinders, state.Working)), t0)
		started := log.Observe(snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working)), t0.Add(time.Minute))
		ended := log.Observe(snapshot(session("b", botropolis, state.Working)), t0.Add(2*time.Minute))

		t.Run("it should log the start", func(t *testing.T) {
			assert.Equal(t, []events.Kind{events.Started}, kinds(started))
		})

		t.Run("it should log the end", func(t *testing.T) {
			require.Len(t, ended, 1)
			assert.Equal(t, events.Ended, ended[0].Kind)
			assert.Equal(t, "a", ended[0].SessionID)
		})

		t.Run("it should list newest first", func(t *testing.T) {
			logged := log.Events()
			require.Len(t, logged, 2)
			assert.Equal(t, events.Ended, logged[0].Kind)
		})

		t.Run("it should answer for a window and kinds", func(t *testing.T) {
			since := log.Since(t0.Add(90*time.Second), events.Ended, events.NeedsYou)
			assert.Equal(t, []events.Kind{events.Ended}, kinds(since))
		})
	})

	t.Run("when the log is long", func(t *testing.T) {
		log := events.NewLog()
		log.Observe(snapshot(session("a", cinders, state.Working)), t0)
		for i := 0; i < events.Keep+20; i++ {
			s := session("a", cinders, state.Working)
			s.APIErrors = i + 1
			log.Observe(snapshot(s), t0.Add(time.Duration(i+1)*time.Second))
		}

		t.Run("it should keep only the newest", func(t *testing.T) {
			assert.Len(t, log.Events(), events.Keep)
		})
	})
}

func TestLogSinceAndAdd(t *testing.T) {
	t.Run("when since is zero", func(t *testing.T) {
		log := events.NewLog()
		log.Observe(snapshot(session("a", cinders, state.Working)), now)
		log.Observe(snapshot(session("a", cinders, state.NeedsYou)), now.Add(time.Minute))

		t.Run("it should return the whole log", func(t *testing.T) {
			assert.Len(t, log.Since(time.Time{}), 1)
		})
	})

	t.Run("when events are added by hand", func(t *testing.T) {
		log := events.NewLog()
		log.Add(events.Event{At: now, Kind: events.Started, Title: "first"})
		log.Add(events.Event{At: now.Add(2 * time.Minute), Kind: events.Error, Title: "newest"},
			events.Event{At: now.Add(time.Minute), Kind: events.NeedsYou, Title: "older"})

		t.Run("it should keep them newest first with a batch in its own order", func(t *testing.T) {
			assert.Equal(t, []events.Kind{events.Error, events.NeedsYou, events.Started}, kinds(log.Events()))
		})
	})
}
