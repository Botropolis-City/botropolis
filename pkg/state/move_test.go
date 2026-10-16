package state_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
)

func TestGroupMove(t *testing.T) {
	limiter := func(st state.State, prs ...claude.PR) state.Session {
		return state.Session{ID: "a", Title: "Fix limiter", State: st, PRs: prs}
	}
	closed := func(prs ...claude.PR) state.Session {
		return state.Session{ID: "b", Title: "Bump client", State: state.Parked, PRs: prs}
	}
	pr := func(n int, st string) claude.PR {
		return claude.PR{Number: n, Repository: "acme/lib", State: st}
	}
	cases := []struct {
		scenario string
		members  []state.Session
		move     state.Move
		why      string
	}{
		{"a member needs you", []state.Session{limiter(state.NeedsYou), closed()}, state.MoveYours, "Fix limiter needs you"},
		{"a member is working", []state.Session{limiter(state.Working), closed()}, state.MoveAgents, "Fix limiter is working"},
		{"a member is working unattended", []state.Session{limiter(state.Unattended), closed()}, state.MoveAgents, "Fix limiter is working"},
		{"a member waits on its own wakeup", []state.Session{limiter(state.Waiting), closed()}, state.MoveAgents, "Fix limiter is working"},
		{"every session is closed and a PR is open", []state.Session{limiter(state.Parked, pr(41, claude.PROpen)), closed()}, state.MoveTheirs, "PR #41 open in acme/lib"},
		{"every session is closed and every PR merged or closed", []state.Session{limiter(state.Parked, pr(41, claude.PRMerged)), closed(pr(42, claude.PRClosed))}, state.MoveNobody, "done: every PR merged or closed"},
		{"every session is closed and there are no PRs", []state.Session{limiter(state.Parked), closed()}, state.MoveNobody, "nothing open"},
		{"members disagree", []state.Session{closed(pr(41, claude.PROpen)), limiter(state.Working), {ID: "c", Title: "Gateway", State: state.NeedsYou}}, state.MoveYours, "Gateway needs you"},
	}
	for _, c := range cases {
		t.Run("when "+c.scenario, func(t *testing.T) {
			move, why := state.GroupMove(c.members)

			t.Run("it should say whose move it is", func(t *testing.T) {
				assert.Equal(t, c.move, move)
			})

			t.Run("it should say why", func(t *testing.T) {
				assert.Equal(t, c.why, why)
			})
		})
	}
}
