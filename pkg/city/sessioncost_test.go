package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

// Item 116: what a building says it spent.
func TestBuildingSpent(t *testing.T) {
	claudeSession := session("a", cinders, state.Working)
	claudeSession.Harness = "claude"
	claudeSession.CostUSD, claudeSession.CostKnown = 9, true
	codexSession := session("x", botropolis, state.Working)
	codexSession.Harness = "codex"
	codexSession.CostUSD, codexSession.CostKnown = 1.5, true
	spent := func(c *city.City, id string) string {
		for _, b := range c.Buildings() {
			if b.Session.ID == id {
				return b.Spent()
			}
		}
		return "no building"
	}

	t.Run("when the store priced a Claude session", func(t *testing.T) {
		c := city.BuildWithLedger(snapshot(claudeSession, codexSession), city.NewLayout(), &fakeLedger{sessions: map[string]stats.Cost{"a": cost(2)}})

		t.Run("it should say the store's cost", func(t *testing.T) {
			assert.Equal(t, "$2.00", spent(c, "a"))
		})

		t.Run("it should leave a Codex session's own figure as it was", func(t *testing.T) {
			assert.Equal(t, format.USD(1.5, true), spent(c, "x"))
		})
	})

	t.Run("when the store has nothing for a Claude session, it should give a dash", func(t *testing.T) {
		c := city.BuildWithLedger(snapshot(claudeSession), city.NewLayout(), &fakeLedger{sessions: map[string]stats.Cost{}})
		assert.Equal(t, format.Unknown, spent(c, "a"))
	})

	t.Run("when statistics are off", func(t *testing.T) {
		c := city.Build(snapshot(claudeSession, codexSession), city.NewLayout())

		t.Run("it should give a Claude session a dash rather than Claude Code's own figure", func(t *testing.T) {
			assert.Equal(t, format.Unknown, spent(c, "a"))
		})

		t.Run("it should leave a Codex session's own figure as it was", func(t *testing.T) {
			assert.Equal(t, format.USD(1.5, true), spent(c, "x"))
		})
	})

	t.Run("when the cost lens totals the city", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetLedger(&fakeLedger{sessions: map[string]stats.Cost{"a": cost(2)}})
		s.SetSnapshot(snapshot(claudeSession, codexSession))

		s.SelectLens(city.LensCost)

		t.Run("it should add the store's cost and Codex's own", func(t *testing.T) {
			assert.Equal(t, "$3.50 across 2 sessions, since they started", s.LensPanel().Aggregate)
		})
	})
}
