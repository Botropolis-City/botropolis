package city_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"github.com/stretchr/testify/assert"
)

func TestSummaryPacked(t *testing.T) {
	t.Run("when sessions are packed up into the depot", func(t *testing.T) {
		crate := session("c", cinders, state.Resting)
		crate.Packed = true
		s := city.Build(snapshot(session("a", cinders, state.Resting), crate), city.NewLayout()).Summary()

		t.Run("it should not count them as resting on the map (item 86)", func(t *testing.T) {
			assert.Equal(t, 1, s.Resting)
		})
	})
}

func TestSummary(t *testing.T) {
	t.Run("when the city has sessions in every state", func(t *testing.T) {
		snap := richSnapshot()
		s := city.Build(snap, city.NewLayout()).Summary()

		t.Run("it should count sessions by state", func(t *testing.T) {
			assert.Equal(t, 1, s.Working)
			assert.Equal(t, 1, s.Unattended)
			assert.Equal(t, 1, s.Resting)
			assert.Equal(t, 2, s.Live())
		})

		t.Run("it should sum token rates over live sessions only", func(t *testing.T) {
			assert.InDelta(t, 152_000, s.FreshPerH, 1e-9)
		})

		t.Run("it should sum output rates apart from fresh ones", func(t *testing.T) {
			assert.InDelta(t, 41_000, s.OutputPerH, 1e-9)
		})

		t.Run("it should carry the plant's hit ratio", func(t *testing.T) {
			assert.InDelta(t, snap.Power.HitRatio(), s.HitRatio, 1e-9)
		})

		t.Run("it should say statistics are off", func(t *testing.T) {
			assert.Equal(t, "statistics are off", s.Reason)
		})

		t.Run("it should sum MCP calls across the towers", func(t *testing.T) {
			assert.Equal(t, 3, s.MCPCalls)
		})

		t.Run("it should count PRs, subagents and errors", func(t *testing.T) {
			assert.Equal(t, 1, s.PRs)
			assert.Equal(t, 2, s.Subagents)
			assert.Equal(t, 2, s.Errors)
		})
	})

	t.Run("when the store priced the day", func(t *testing.T) {
		s := city.BuildWithLedger(snapshot(session("a", cinders, state.Working)), city.NewLayout(), &fakeLedger{window: stored}).Summary()

		t.Run("it should carry the day's cost", func(t *testing.T) {
			assert.Equal(t, stored.Cost, s.Cost)
		})

		t.Run("it should be priced", func(t *testing.T) {
			assert.True(t, s.Priced)
		})
	})

	t.Run("when the store cannot be read", func(t *testing.T) {
		s := city.BuildWithLedger(snapshot(session("a", cinders, state.Working)), city.NewLayout(), &fakeLedger{reason: "the store is being upgraded"}).Summary()

		t.Run("it should carry the reason", func(t *testing.T) {
			assert.Equal(t, "the store is being upgraded", s.Reason)
		})
	})

	t.Run("when a headline is asked for", func(t *testing.T) {
		cases := map[string]struct {
			s    city.Summary
			want string
		}{
			"with sessions needing you": {city.Summary{NeedsYou: 2, Working: 1, Unattended: 1}, "2 need you · 1 working · 1 unattended"},
			"with only work running":    {city.Summary{Working: 3}, "3 working"},
			"with only unattended work": {city.Summary{Unattended: 1, Resting: 2}, "1 unattended"},
			"with nothing live":         {city.Summary{Resting: 9}, "quiet"},
		}
		for name, c := range cases {
			t.Run("and the city is "+name, func(t *testing.T) {
				t.Run("it should say so in one line", func(t *testing.T) {
					assert.Equal(t, c.want, c.s.Headline())
				})
			})
		}
	})

	t.Run("when the needs-you chip is hovered", func(t *testing.T) {
		a := session("a", cinders, state.NeedsYou)
		a.Tool, a.Subject = "Bash", "make test"
		card := city.Build(snapshot(a, session("b", botropolis, state.Working)), city.NewLayout()).StateCard(state.NeedsYou)

		t.Run("it should list the sessions in that state with what they are doing", func(t *testing.T) {
			assert.Equal(t, "needs-you", card.Title)
			assert.Equal(t, []string{"Fix the CI queue  Bash make test"}, card.Lines)
		})
	})

	t.Run("when a chip for an empty state is hovered", func(t *testing.T) {
		card := city.Build(snapshot(session("a", cinders, state.Working)), city.NewLayout()).StateCard(state.Resting)

		t.Run("it should say none", func(t *testing.T) {
			assert.Equal(t, []string{"none"}, card.Lines)
		})
	})

	t.Run("when the city is empty", func(t *testing.T) {
		t.Run("it should count nothing, and say statistics are off", func(t *testing.T) {
			assert.Equal(t, city.Summary{Reason: stats.ReasonOff}, city.Build(state.Snapshot{}, city.NewLayout()).Summary())
		})
	})
}
