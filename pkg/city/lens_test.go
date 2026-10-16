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

// connected is a city where every lens's first action has something to
// act on: a grouped session with a pull request, in a district with a
// road to another.
func connected(t *testing.T) *city.Scene {
	t.Helper()
	a := session("a", cinders, state.NeedsYou)
	a.Group = "g1"
	a.PRs = []claude.PR{{Number: 882, URL: "https://example.com/pr/882", Repository: "o/api", State: claude.PROpen}}
	b := session("b", botropolis, state.Working)
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{a, b},
		Groups: []state.Group{{ID: "g1", Name: "Rate-limit headers", Members: []string{"a"}, Move: state.MoveYours}},
		Roads:  []state.Road{{From: cinders, To: botropolis, Messages: 3}}})
	s.Click(centreOf(t, s, "a"))
	require.NotNil(t, s.Selected())
	return s
}

func TestLenses(t *testing.T) {
	t.Run("it should fold the views into six lenses, every view in exactly one", func(t *testing.T) {
		count := map[city.View]int{}
		for _, l := range city.Lenses {
			for _, v := range l.Views() {
				count[v]++
			}
		}
		want := map[city.View]int{}
		for _, v := range city.Views {
			want[v] = 1
		}
		assert.Equal(t, []any{6, want}, []any{len(city.Lenses), count})
	})

	t.Run("for every saved view name", func(t *testing.T) {
		for _, v := range city.Views {
			t.Run("when it is "+v.Name(), func(t *testing.T) {
				layout := city.NewLayout()
				layout.View = v.Name()
				s := city.NewScene(layout)

				t.Run("it should open the lens holding it, on that view", func(t *testing.T) {
					assert.Equal(t, []any{city.LensOf(v), v}, []any{s.Lens(), s.View()})
				})
			})
		}
	})

	t.Run("when a lens's key is pressed again", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.SelectLens(city.LensCapacity)
		first := s.View()
		s.SelectLens(city.LensCapacity)
		second := s.View()
		s.SelectLens(city.LensCapacity)

		t.Run("it should step to its next view, wrapping", func(t *testing.T) {
			assert.Equal(t, []city.View{city.ViewPressure, city.ViewFanout, city.ViewPressure}, []city.View{first, second, s.View()})
		})
	})

	t.Run("for every lens", func(t *testing.T) {
		leads := map[city.Lens]string{
			city.LensAttention:   "context",
			city.LensWork:        "station",
			city.LensCost:        "spent",
			city.LensCapacity:    "context",
			city.LensHealth:      "errors",
			city.LensConnections: "server calls",
		}
		firsts := map[city.Lens]city.ActionKind{
			city.LensAttention:   city.ActionAttach,
			city.LensWork:        city.ActionFocusLine,
			city.LensCost:        city.ActionBreakdown,
			city.LensCapacity:    city.ActionFork,
			city.LensHealth:      city.ActionOpenPR,
			city.LensConnections: city.ActionShowRoad,
		}
		for _, l := range city.Lenses {
			t.Run("when it is "+l.Name(), func(t *testing.T) {
				s := connected(t)
				s.SelectLens(l)
				card, _, _ := s.SelectedCard()

				t.Run("it should lead the card with that lens's number", func(t *testing.T) {
					require.NotEmpty(t, card.Numbers)
					assert.Equal(t, leads[l], card.Numbers[0].Label)
				})

				t.Run("it should put that lens's first action first", func(t *testing.T) {
					assert.Equal(t, firsts[l], s.Actions()[0])
				})

				t.Run("it should have a one-sentence description and a what-to-do line", func(t *testing.T) {
					assert.True(t, l.About() != "" && l.Do() != "")
				})
			})
		}
	})

	t.Run("when Capacity is up", func(t *testing.T) {
		s := connected(t)
		s.SelectLens(city.LensCapacity)

		t.Run("it should offer a new session here beside the fork", func(t *testing.T) {
			assert.Equal(t, city.ActionNew, s.Actions()[1])
		})
	})

	t.Run("when a lens's first action is pressed", func(t *testing.T) {
		t.Run("and it is fork, it should ask to fork the session in its own directory", func(t *testing.T) {
			action, _ := connected(t).Act(city.ActionFork)
			assert.Equal(t, city.Action{Kind: city.ActionFork, SessionID: "a", Dir: cinders}, action)
		})

		t.Run("and it is open pull request, it should ask to open the pull request's address", func(t *testing.T) {
			action, _ := connected(t).Act(city.ActionOpenPR)
			assert.Equal(t, city.Action{Kind: city.ActionOpenPR, Text: "https://example.com/pr/882"}, action)
		})

		t.Run("and it is breakdown, it should ask for the breakdown", func(t *testing.T) {
			action, _ := connected(t).Act(city.ActionBreakdown)
			assert.Equal(t, city.ActionBreakdown, action.Kind)
		})

		t.Run("and it is focus the line, it should focus the session's group", func(t *testing.T) {
			s := connected(t)
			s.Act(city.ActionFocusLine)
			assert.Equal(t, "g1", s.FocusedGroup())
		})

		t.Run("and it is show the road, it should light the district at the road's other end", func(t *testing.T) {
			s := connected(t)
			s.Act(city.ActionShowRoad)
			assert.True(t, s.Highlight().HasBuilding(buildingOf(t, s, "b")))
		})
	})
}

func TestLensPanel(t *testing.T) {
	pressured := func(id string, percent float64) state.Session {
		s := session(id, cinders, state.Working)
		s.ContextPercent = percent
		return s
	}
	busy := []state.Session{pressured("a", 85), pressured("b", 90), pressured("c", 95), pressured("d", 40)}

	t.Run("when a lens opens for the first time", func(t *testing.T) {
		s := scene(t, busy...)
		s.SelectLens(city.LensCapacity)
		panel := s.LensPanel()

		t.Run("it should show its panel", func(t *testing.T) {
			assert.Equal(t, []any{"Capacity", false}, []any{panel.Name, panel.Chip})
		})

		t.Run("it should say the aggregate, counted from the city", func(t *testing.T) {
			assert.Equal(t, "3 sessions over 80%", panel.Aggregate)
		})
	})

	t.Run("when it has been seen three times, and it is opened again", func(t *testing.T) {
		s := scene(t, busy...)
		for range 4 {
			s.SelectLens(city.LensCapacity)
			s.SelectLens(city.LensAttention)
		}
		s.SelectLens(city.LensCapacity)

		t.Run("it should shrink to a chip", func(t *testing.T) {
			assert.True(t, s.LensPanel().Chip)
		})

		t.Run("and v is pressed", func(t *testing.T) {
			s.ShowLensPanel()

			t.Run("it should show the panel again", func(t *testing.T) {
				assert.False(t, s.LensPanel().Chip)
			})
		})
	})

	t.Run("when Botropolis restarts", func(t *testing.T) {
		path := t.TempDir() + "/layout.json"
		layout := city.NewLayout()
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetSnapshot(snapshot(busy...))
		for range 4 {
			s.SelectLens(city.LensCost)
			s.SelectLens(city.LensAttention)
		}
		require.NoError(t, layout.Save(path))
		loaded, err := city.LoadLayout(path)
		require.NoError(t, err)
		again := city.NewScene(loaded)
		again.SetClock(func() time.Time { return now }, time.UTC)
		again.SetSnapshot(snapshot(busy...))
		again.SelectLens(city.LensCost)

		t.Run("it should remember how often each lens was seen", func(t *testing.T) {
			assert.True(t, again.LensPanel().Chip)
		})
	})
}
