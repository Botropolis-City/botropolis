package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestCamps(t *testing.T) {
	lead := session("lead", cinders, state.Working)
	lead.Team = "review"
	worker := session("w1", botropolis, state.Working)
	worker.Team, worker.Agent = "review", "reviewer"
	loner := session("x", "/p/x", state.Working)
	snap := snapshot(lead, worker, loner)
	snap.Teams = []claude.Team{{Name: "review", LeadSessionID: "lead"}}
	c := city.Build(snap, city.NewLayout())

	t.Run("when a team's sessions are on the map", func(t *testing.T) {
		require.Len(t, c.Camps, 1)
		camp := c.Camps[0]

		t.Run("it should make one camp named after the team", func(t *testing.T) {
			assert.Equal(t, "review", camp.Name)
		})

		t.Run("it should know the lead from the roster", func(t *testing.T) {
			require.NotNil(t, camp.Lead)
			assert.Equal(t, "lead", camp.Lead.Session.ID)
		})

		t.Run("it should tie the members to it", func(t *testing.T) {
			require.Len(t, camp.Members, 1)
			assert.Equal(t, "w1", camp.Members[0].Session.ID)
		})

		t.Run("it should find a building's camp", func(t *testing.T) {
			assert.Equal(t, camp, c.Camp(camp.Members[0]))
			assert.Nil(t, c.Camp(c.Districts[2].Buildings[0]))
		})

		t.Run("it should describe the crew on its card", func(t *testing.T) {
			card := camp.Card(c)
			assert.Equal(t, "team review", card.Title)
			assert.Contains(t, card.Lines, "lead     Fix the CI queue")
			assert.Contains(t, card.Lines, "members  reviewer")
		})

		t.Run("it should say the team on a member's card", func(t *testing.T) {
			assert.Contains(t, camp.Members[0].Card(now).Details, "team     review (reviewer)")
			assert.Contains(t, camp.Lead.Card(now).Details, "team     review (lead)")
		})
	})

	t.Run("when sessions name a team the rosters do not", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.Team = "adhoc"
		b := session("b", cinders, state.Working)
		b.Team, b.Agent = "adhoc", "helper"
		c := build(t, city.NewLayout(), a, b)
		require.Len(t, c.Camps, 1)

		t.Run("it should take the session without an agent name as the lead", func(t *testing.T) {
			require.NotNil(t, c.Camps[0].Lead)
			assert.Equal(t, "a", c.Camps[0].Lead.Session.ID)
		})
	})
}

func TestPlantBudget(t *testing.T) {
	t.Run("when a daily budget is set", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		snap := snapshot(session("a", cinders, state.Working))
		s.SetLedger(&fakeLedger{window: stored})
		s.SetSnapshot(snap)
		s.SetBudget(250)

		t.Run("it should measure the plant against it", func(t *testing.T) {
			assert.Contains(t, s.City().Plant.Card().Lines, "budget   $250/day, 1% used")
		})

		t.Run("it should hand the budget to the summary", func(t *testing.T) {
			assert.Equal(t, 250.0, s.City().Summary().BudgetUSD)
		})

		t.Run("and a new snapshot arrives", func(t *testing.T) {
			s.SetSnapshot(snapshot(session("a", cinders, state.Working)))

			t.Run("it should keep the budget", func(t *testing.T) {
				assert.Equal(t, 250.0, s.City().Plant.BudgetUSD)
			})
		})
	})
}
