package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86, slice 5: the journal answers where you were and what is next,
// one entry per group and per loose session, from what Claude already
// wrote.
func TestJournal(t *testing.T) {
	quiet := func(id, cwd string, ago time.Duration) state.Session {
		s := session(id, cwd, state.Resting)
		s.LastActivity, s.Recap, s.LastPrompt, s.Note = now.Add(-ago), claude.Recap{}, "", ""
		return s
	}
	grouped := quiet("g1", cinders, 3*time.Hour)
	grouped2 := quiet("g2", botropolis, time.Hour)
	loose := quiet("l1", mullet, 2*time.Hour)
	snap := state.Snapshot{At: now, Sessions: []state.Session{grouped, grouped2, loose},
		Groups: []state.Group{{ID: "grp", Name: "Release", Members: []string{"g1", "g2"}}}}
	noReply := func(string) string { return "" }
	titles := func(entries []city.JournalEntry) []string {
		var out []string
		for _, e := range entries {
			out = append(out, e.Title)
		}
		return out
	}

	t.Run("when groups and loose sessions are on the map", func(t *testing.T) {
		entries := city.BuildJournal(snap, nil, noReply)

		t.Run("it should make one entry per group", func(t *testing.T) {
			assert.Contains(t, titles(entries), "Release")
		})

		t.Run("it should make one entry per loose session", func(t *testing.T) {
			assert.Len(t, entries, 2)
		})

		t.Run("it should order them most recently touched first", func(t *testing.T) {
			assert.Equal(t, []string{"Release", loose.Title}, titles(entries))
		})

		t.Run("each entry should name where it is", func(t *testing.T) {
			assert.Equal(t, []string{"botropolis", "cinders"}, entries[0].Where)
		})

		t.Run("each entry should fly to its most recently touched session", func(t *testing.T) {
			assert.Equal(t, "g2", entries[0].Fly)
		})
	})

	t.Run("when a session was pinned at close up", func(t *testing.T) {
		entries := city.BuildJournal(snap, []string{"l1"}, noReply)

		t.Run("its entry should come first", func(t *testing.T) {
			assert.Equal(t, "l1", entries[0].Fly)
		})

		t.Run("it should say it was pinned", func(t *testing.T) {
			assert.True(t, entries[0].Pinned)
		})
	})

	for _, c := range []struct {
		name               string
		recap, note, reply string
		prompt             string
		last, next         string
	}{
		{"and it has a recap with a next line", "Shipped the parser.\nNext: tag v1.4.0", "", "", "", "Shipped the parser.", "tag v1.4.0"},
		{"and it has a recap with no next line, and asked a question", "Shipped the parser.", "Which branch?", "", "", "Shipped the parser.", "Which branch?"},
		{"and it has no recap, and its reply ends in a question", "", "", "It builds now.\n\nShall I tag it?", "make it build", "You asked: “make it build” — Shall I tag it?", "Shall I tag it?"},
		{"and it has nothing to go on", "", "", "", "make it build", "You asked: “make it build”", "not recorded"},
	} {
		t.Run("when a loose session is journaled "+c.name, func(t *testing.T) {
			s := quiet("x", mullet, time.Hour)
			s.Recap.Text, s.Note, s.LastPrompt = c.recap, c.note, c.prompt
			if c.note != "" {
				s.State = state.NeedsYou
			}
			entries := city.BuildJournal(state.Snapshot{At: now, Sessions: []state.Session{s}}, nil, func(string) string { return c.reply })
			require.Len(t, entries, 1)

			t.Run("its last time should be "+c.last, func(t *testing.T) {
				assert.Equal(t, c.last, entries[0].LastTime)
			})

			t.Run("its next should be "+c.next, func(t *testing.T) {
				assert.Equal(t, c.next, entries[0].Next)
			})
		})
	}

	t.Run("when sessions are empty or packed up", func(t *testing.T) {
		crate := quiet("c", mullet, time.Hour)
		crate.Packed = true
		empty := quiet("e", mullet, time.Hour)
		empty.State = state.Empty
		entries := city.BuildJournal(state.Snapshot{At: now, Sessions: []state.Session{crate, empty}}, nil, noReply)

		t.Run("it should leave them out", func(t *testing.T) {
			assert.Empty(t, entries)
		})
	})
}

func TestLastSight(t *testing.T) {
	t.Run("when the map is seen", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", cinders, state.Resting), session("c", botropolis, state.NeedsYou))
		s.NoteSeen(now)

		t.Run("it should note when", func(t *testing.T) {
			assert.Equal(t, now, s.Layout().Seen)
		})

		t.Run("it should note the ids of its live sessions", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"a", "c"}, s.Layout().LastLive)
		})
	})
}
