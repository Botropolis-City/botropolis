package city_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func richSnapshot() state.Snapshot {
	a := session("a", cinders, state.Working)
	a.MCPCalls = map[string]int{"atlassian": 3}
	a.PRs = []claude.PR{{Number: 1181, URL: "https://github.com/mCedar/mullet/pull/1181"}}
	a.APIErrors = 2
	b := session("b", botropolis, state.Unattended)
	b.FreshTokensPerHour, b.CacheReadPerHour = 0, 0
	c := session("c", cinders, state.Resting)
	return state.Snapshot{
		At:       now,
		Sessions: []state.Session{a, b, c},
		Servers: []state.Server{
			{Name: "atlassian", Calls: 3, Sessions: 1},
			{Name: "datadog-mcp", Type: "http", Configured: true},
		},
		Skills: []state.Skill{{Name: "amberPylon:umberEstuary", Calls: 3, Sessions: 2}, {Name: "git-worktrees", Calls: 1, Sessions: 1}},
		Power: state.Power{Since: now.Add(-24 * time.Hour), CostUSD: 12.5, CostKnown: true, Fresh: 30_000, Cached: 900_000,
			ByModel: map[string]claude.Usage{"claude-opus-5[1m]": {Output: 20_000}}},
		Stats: &claude.Stats{
			LastComputed:   now.Add(-48 * time.Hour).Format("2006-01-02"),
			FirstSessionAt: time.Date(2026, time.January, 13, 17, 49, 58, 0, time.UTC),
			TotalSessions:  513,
			TotalMessages:  125894,
			DailyActivity: []claude.DailyActivity{
				{Date: now.Add(-72 * time.Hour).Format("2006-01-02"), Messages: 100, Sessions: 1},
				{Date: now.Add(-48 * time.Hour).Format("2006-01-02"), Messages: 300, Sessions: 3},
			},
			ModelUsage: map[string]claude.ModelCost{"claude-opus-5": {Usage: claude.Usage{Output: 750}}, "claude-sonnet-5": {Usage: claude.Usage{Output: 250}}},
			HourCounts: [24]int{9: 40, 14: 41},
		},
	}
}

func TestHall(t *testing.T) {
	t.Run("when the snapshot carries Claude Code's stats rollup", func(t *testing.T) {
		c := city.Build(richSnapshot(), city.NewLayout())
		card := c.Hall.Card(now)

		t.Run("it should stand on the plaza, right of the plant", func(t *testing.T) {
			assert.True(t, c.Plaza.Contains(c.Hall.Rect.Center()))
			assert.Greater(t, c.Hall.Rect.Min.X, c.Plant.Rect.Max.X)
		})

		t.Run("it should be part of the city bounds", func(t *testing.T) {
			assert.True(t, c.Bounds().Contains(c.Hall.Rect.Center()))
		})

		t.Run("it should be hit at its centre", func(t *testing.T) {
			assert.Equal(t, city.LandmarkHall, c.At(c.Hall.Rect.Center()).Landmark)
		})

		t.Run("it should say how stale the rollup is", func(t *testing.T) {
			assert.Contains(t, card.Lines, "computed "+now.Add(-48*time.Hour).Format("2006-01-02")+" (2 days ago)")
		})

		t.Run("it should show the totals", func(t *testing.T) {
			assert.Contains(t, card.Lines, "sessions 513 since 2026-01-13")
		})

		t.Run("it should average the recent days", func(t *testing.T) {
			assert.Contains(t, card.Lines, "last 2 days  200 msgs/day, 4 sessions")
		})

		t.Run("it should name the busiest hour", func(t *testing.T) {
			assert.Contains(t, card.Lines, "busiest  14:00")
		})

		t.Run("it should show the model mix", func(t *testing.T) {
			assert.Contains(t, card.Lines, "claude-opus-5  75%")
		})
	})

	t.Run("when the snapshot has no rollup", func(t *testing.T) {
		c := city.Build(snapshot(session("a", cinders, state.Working)), city.NewLayout())

		t.Run("it should build no hall", func(t *testing.T) {
			assert.Zero(t, c.Hall.Rect.Area())
		})
	})
}

func TestLandmarks(t *testing.T) {
	c := city.Build(richSnapshot(), city.NewLayout())
	require.NotNil(t, c)

	t.Run("when the city is built from a snapshot with power, servers and skills", func(t *testing.T) {
		t.Run("it should put the power plant on the plaza", func(t *testing.T) {
			assert.True(t, c.Plaza.Contains(c.Plant.Rect.Min) && c.Plaza.Contains(c.Plant.Rect.Max))
		})

		t.Run("it should keep the plaza clear of every district", func(t *testing.T) {
			for _, d := range c.Districts {
				assert.False(t, d.Rect.Overlaps(c.Plaza), d.Name)
			}
		})

		t.Run("it should raise one tower per server along the north ridge", func(t *testing.T) {
			require.Len(t, c.Towers, 2)
			for _, tower := range c.Towers {
				assert.LessOrEqual(t, tower.Rect.Max.Y, c.Plaza.Min.Y, tower.Server.Name)
				for _, d := range c.Districts {
					assert.LessOrEqual(t, tower.Rect.Max.Y, d.Rect.Min.Y, tower.Server.Name)
				}
			}
			assert.Less(t, c.Towers[0].Rect.Min.X, c.Towers[1].Rect.Min.X)
		})

		t.Run("it should put the library on the plaza below the plant", func(t *testing.T) {
			assert.True(t, c.Plaza.Contains(c.Library.Rect.Center()))
			assert.Greater(t, c.Library.Rect.Min.Y, c.Plant.Rect.Max.Y)
		})

		t.Run("it should include the landmarks in the bounds", func(t *testing.T) {
			assert.True(t, c.Bounds().Contains(c.Plant.Rect.Min))
			assert.True(t, c.Bounds().Contains(c.Towers[0].Rect.Min))
			assert.True(t, c.Bounds().Contains(c.Library.Rect.Max))
		})

		t.Run("it should start the map at the origin", func(t *testing.T) {
			assert.Equal(t, city.Point{}, c.Bounds().Min)
		})
	})

	t.Run("when lines are drawn from the plant", func(t *testing.T) {
		lines := c.PowerLines()

		t.Run("it should run one line to each lit building that draws tokens", func(t *testing.T) {
			require.Len(t, lines, 1)
			assert.Equal(t, "a", lines[0].Building.Session.ID)
		})

		t.Run("it should start at the plant and end at the building", func(t *testing.T) {
			assert.Equal(t, c.Plant.Rect.Center(), lines[0].From)
			assert.Equal(t, lines[0].Building.Rect.Center(), lines[0].To)
		})

		t.Run("it should carry both rates", func(t *testing.T) {
			assert.InDelta(t, 152_000, lines[0].Fresh, 1e-9)
			assert.InDelta(t, 8_900_000, lines[0].Cached, 1e-9)
		})
	})

	t.Run("when beams are drawn from the towers", func(t *testing.T) {
		beams := c.Beams()

		t.Run("it should run one beam per server a session called", func(t *testing.T) {
			require.Len(t, beams, 1)
			assert.Equal(t, "atlassian", beams[0].Tower.Server.Name)
			assert.Equal(t, "a", beams[0].Building.Session.ID)
			assert.Equal(t, 3, beams[0].Calls)
		})
	})

	t.Run("when a session is running unattended", func(t *testing.T) {
		t.Run("it should not make it night: the clock does that", func(t *testing.T) {
			assert.False(t, c.Night)
		})
	})

	t.Run("when a building's session has PRs and errors", func(t *testing.T) {
		var a *city.Building
		for _, b := range c.Buildings() {
			if b.Session.ID == "a" {
				a = b
			}
		}
		require.NotNil(t, a)

		t.Run("it should fly a flag", func(t *testing.T) {
			assert.Equal(t, 1, a.Flags)
		})

		t.Run("it should give off smoke", func(t *testing.T) {
			assert.Equal(t, 2, a.Smoke)
		})
	})

	t.Run("when the plant's cost is unknown", func(t *testing.T) {
		unpriced := c.Plant
		unpriced.Power.CostUSD, unpriced.Power.CostKnown = 0, false
		unpriced.BudgetUSD = 50
		card := unpriced.Card()

		t.Run("it should show a dash and say why", func(t *testing.T) {
			assert.Contains(t, card.Lines, "cost     \u2014 in 24h (no cost-state in any transcript)")
		})

		t.Run("it should not measure a budget it cannot", func(t *testing.T) {
			for _, line := range card.Lines {
				assert.NotContains(t, line, "budget")
			}
		})
	})

	t.Run("when landmarks are hovered", func(t *testing.T) {
		t.Run("it should describe the plant with cost and tokens", func(t *testing.T) {
			card := c.Plant.Card()
			assert.Equal(t, "Power plant", card.Title)
			assert.Contains(t, card.Lines, "cost     ~$12.50 in 24h (pro-rated)")
			assert.Contains(t, card.Lines, "fresh    30k tokens")
			assert.Contains(t, card.Lines, "cached   900k tokens")
			assert.Contains(t, card.Lines, "hit rate 97% of prompt tokens from cache")
			assert.Contains(t, card.Lines, "claude-opus-5[1m]  20k out")
		})

		t.Run("it should describe a tower with its calls", func(t *testing.T) {
			card := c.Towers[0].Card()
			assert.Equal(t, "atlassian", card.Title)
			assert.Contains(t, card.Lines, "calls    3 in 1 session")
			assert.Contains(t, card.Lines, "config   used, not configured")
		})

		t.Run("it should describe the library with its top skills", func(t *testing.T) {
			card := c.Library.Card()
			assert.Equal(t, "Library", card.Title)
			assert.Contains(t, card.Lines, "amberPylon:umberEstuary  3 calls, 2 sessions")
		})

		t.Run("it should find the plant under the pointer", func(t *testing.T) {
			hit := c.At(c.Plant.Rect.Center())
			assert.Equal(t, city.LandmarkPlant, hit.Landmark)
		})

		t.Run("it should find a tower under the pointer", func(t *testing.T) {
			hit := c.At(c.Towers[1].Rect.Center())
			assert.Equal(t, city.LandmarkTower, hit.Landmark)
			require.NotNil(t, hit.Tower)
			assert.Equal(t, "datadog-mcp", hit.Tower.Server.Name)
		})
	})
}

func TestLandmarksWithoutData(t *testing.T) {
	t.Run("when the snapshot has no servers or skills", func(t *testing.T) {
		c := city.Build(snapshot(session("a", cinders, state.Working)), city.NewLayout())

		t.Run("it should raise no towers", func(t *testing.T) {
			assert.Empty(t, c.Towers)
		})

		t.Run("it should still place the plant", func(t *testing.T) {
			assert.Greater(t, c.Plant.Rect.Area(), 0.0)
		})
	})

	t.Run("when the snapshot is empty", func(t *testing.T) {
		c := city.Build(state.Snapshot{}, city.NewLayout())

		t.Run("it should have no plant", func(t *testing.T) {
			assert.Zero(t, c.Plant.Rect.Area())
		})
	})
}

func TestRoads(t *testing.T) {
	snap := snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working), session("c", worktree, state.Resting))
	snap.Roads = []state.Road{
		{From: cinders, To: botropolis, Messages: 3, Sessions: 1},
		{From: worktree, To: cinders, Messages: 2, Files: 4, Sessions: 1},
		{From: cinders, To: "/somewhere/unknown", Messages: 9, Sessions: 1},
	}
	c := city.Build(snap, city.NewLayout())

	t.Run("when the snapshot has roads between projects", func(t *testing.T) {
		roads := c.Roads

		t.Run("it should draw one road per district pair that exists, whichever way it was crossed", func(t *testing.T) {
			require.Len(t, roads, 1)
		})

		t.Run("it should fold worktrees into their project's district", func(t *testing.T) {
			assert.Equal(t, 4, roads[0].BackFiles)
		})

		t.Run("it should keep the other way's messages apart", func(t *testing.T) {
			assert.Equal(t, 2, roads[0].BackMessages)
		})

		t.Run("it should run between district centres", func(t *testing.T) {
			assert.Equal(t, roads[0].From.Rect.Center(), roads[0].A)
			assert.Equal(t, roads[0].To.Rect.Center(), roads[0].B)
		})

		t.Run("it should carry the message count", func(t *testing.T) {
			assert.Equal(t, 3, roads[0].Messages)
		})

		t.Run("it should sign each road with its traffic", func(t *testing.T) {
			assert.Equal(t, "5 msgs, 4 files", roads[0].Label())
		})

		t.Run("it should sign a lone touch in the singular", func(t *testing.T) {
			assert.Equal(t, "1 msg, 1 file", city.RoadLine{Messages: 1, Files: 1}.Label())
		})
	})

	t.Run("when a district with roads is hovered", func(t *testing.T) {
		var ciq *city.District
		for _, d := range c.Districts {
			if d.Name == "cinders" {
				ciq = d
			}
		}
		require.NotNil(t, ciq)
		card := ciq.Card()

		t.Run("it should list the roads in and out", func(t *testing.T) {
			assert.Contains(t, card.Details, "roads    3 msgs to botropolis, 2 msgs, 4 files from botropolis")
		})
	})
}
