package group_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/group"
)

// Item 90: a session whose subagents work in another session's
// directory while that session runs is coordinating it, the strongest
// signal.
func TestCoordinates(t *testing.T) {
	// foreman's subagents worked in lib and api from 4h to 6h in.
	foreman := func() group.Member {
		m := member("f", "/w/meta", 0, "build the headers across the repositories")
		m.Visited = []group.Visit{
			{Dir: "/w/lib", From: t0.Add(4 * time.Hour), To: t0.Add(6 * time.Hour)},
			{Dir: "/w/api", From: t0.Add(4 * time.Hour), To: t0.Add(6 * time.Hour)},
		}
		return m
	}
	joined := func(r group.Result, a, b string) bool {
		g, ok := r.GroupOf(a)
		return ok && g.Member(b).Session != ""
	}

	t.Run("when a session's subagents worked in another session's directory while it ran", func(t *testing.T) {
		r := detect(group.Memory{}, foreman(), member("l", "/w/lib", 5*time.Hour, "fix the flaky clock"))

		t.Run("they should join one group", func(t *testing.T) {
			assert.True(t, joined(r, "f", "l"))
		})

		t.Run("the reason should be that it coordinates it, a sure signal", func(t *testing.T) {
			g, ok := r.GroupOf("l")
			require.True(t, ok)
			m := g.Member("l")
			assert.Equal(t, [2]any{group.SignalCoordinates, group.Sure}, [2]any{m.Reason.Signal, m.Tier})
		})

		t.Run("and memory marks them not related", func(t *testing.T) {
			r := detect(group.Memory{NotRelated: []group.Pair{{"f", "l"}}}, foreman(), member("l", "/w/lib", 5*time.Hour, "fix the flaky clock"))

			t.Run("they should not join", func(t *testing.T) {
				assert.False(t, joined(r, "f", "l"))
			})
		})
	})

	for _, c := range []struct {
		when    string
		started time.Duration
		ended   time.Duration
	}{
		{"the other session ended before the subagents arrived", -3 * time.Hour, -2 * time.Hour},
		{"the other session started after the subagents left, while the coordinator ran on", 30 * time.Hour, 0},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			l := member("l", "/w/lib", c.started, "fix the flaky clock")
			if c.ended != 0 {
				l.Ended = t0.Add(c.ended)
			}
			r := detect(group.Memory{}, foreman(), l)

			t.Run("it should not be read as coordinating", func(t *testing.T) {
				assert.False(t, joined(r, "f", "l"))
			})
		})
	}

	t.Run("when it also shares a reference with the session it coordinates", func(t *testing.T) {
		f := foreman()
		f.Prompts = []string{"PROJ-5 across the repositories"}
		r := detect(group.Memory{}, f, member("l", "/w/lib", 5*time.Hour, "PROJ-5 in the library"))
		g, ok := r.GroupOf("l")
		require.True(t, ok)

		t.Run("coordinates should be the stronger reason", func(t *testing.T) {
			assert.Equal(t, group.SignalCoordinates, g.Member("l").Reason.Signal)
		})
	})
}
