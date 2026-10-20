package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

func TestLensPanelWords(t *testing.T) {
	panel := func(lens city.Lens, snap state.Snapshot, ledger city.Ledger) city.LensPanel {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		if ledger != nil {
			s.SetLedger(ledger)
		}
		s.SetSnapshot(snap)
		s.SelectLens(lens)
		return s.LensPanel()
	}
	waiting := func(n int) state.Snapshot {
		sessions := []state.Session{session("w", cinders, state.Working)}
		for _, id := range []string{"a", "b", "c"}[:n] {
			sessions = append(sessions, session(id, cinders, state.NeedsYou))
		}
		return snapshot(sessions...)
	}

	t.Run("when Attention is up", func(t *testing.T) {
		cases := []struct {
			name, count, hint string
			waiting           int
		}{
			{"and nobody needs you", "0 sessions need you", "Nobody is waiting on you; tab finds the next one when someone is.", 0},
			{"and one session needs you", "1 session needs you", "Press tab to go to the next session that needs you.", 1},
			{"and two sessions need you", "2 sessions need you", "Press tab to go to the next session that needs you.", 2},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				p := panel(city.LensAttention, waiting(c.waiting), nil)

				t.Run("it should count the sessions that need you", func(t *testing.T) {
					assert.Equal(t, c.count, p.Aggregate)
				})

				t.Run("it should say what tab does for that count", func(t *testing.T) {
					assert.Equal(t, c.hint, p.Do)
				})
			})
		}
	})

	t.Run("when Work is up", func(t *testing.T) {
		group := func(id string, move state.Move) state.Group {
			return state.Group{ID: id, Name: id, Members: []string{"a"}, Move: move}
		}
		cases := []struct {
			name, want string
			groups     []state.Group
		}{
			{"and there are no groups, it should say so", "no groups yet", nil},
			{"and its one group is not your move, it should say none of one is", "0 of 1 group is your move",
				[]state.Group{group("g1", state.MoveAgents)}},
			{"and one of two groups is your move, it should count groups in the cards' words", "1 of 2 groups are your move",
				[]state.Group{group("g1", state.MoveYours), group("g2", state.MoveAgents)}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				snap := snapshot(session("a", cinders, state.NeedsYou), session("b", cinders, state.NeedsYou))
				snap.Groups = c.groups
				assert.Equal(t, c.want, panel(city.LensWork, snap, nil).Aggregate)
			})
		}
	})

	t.Run("when Cost is up", func(t *testing.T) {
		two := snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working))
		cases := []struct {
			name, want string
			costs      map[string]stats.Cost
		}{
			{"and a session's spend is a range, it should say up to its high end, since they started",
				"up to $1.32 across 2 sessions, since they started",
				map[string]stats.Cost{"a": {Low: 0.5, High: 0.75}, "b": cost(0.57)}},
			{"and every spend is exact, it should say the sum since they started",
				"$1.32 across 2 sessions, since they started",
				map[string]stats.Cost{"a": cost(0.75), "b": cost(0.57)}},
			{"and no spend is known yet, it should say so",
				"spend not known yet", nil},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				assert.Equal(t, c.want, panel(city.LensCost, two, &fakeLedger{sessions: c.costs}).Aggregate)
			})
		}
	})
}

func TestNothingToJumpTo(t *testing.T) {
	cases := map[state.State]string{
		state.NeedsYou: "nobody needs you right now",
		state.Working:  "nothing is working",
	}
	for st, want := range cases {
		t.Run("when tab finds nothing "+string(st)+", it should say so in words", func(t *testing.T) {
			assert.Equal(t, want, city.NothingToJumpTo(st))
		})
	}
}
