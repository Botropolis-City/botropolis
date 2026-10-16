package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 90. Focusing a group docks a list of its stations beside the
// map, away from the river, stepped with j and k.

func TestDive(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	const width, height = 1280.0, 800.0
	th := ui.NewTheme(1)
	measure := func(s string, _ ui.Size) (float64, float64) { return 7 * float64(len([]rune(s))), 14 }
	member := func(id, cwd string, started time.Duration) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.Working, Group: "g1",
			StartedAt: now.Add(started), LastActivity: now.Add(started / 2)}
	}
	newGame := func(t *testing.T, turns int, sidebar bool) *Game {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(width, height)
		s.SetSnapshot(state.Snapshot{At: now, Groups: []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"a", "b", "c"}}},
			Sessions: []state.Session{
				member("a", "/w/lib", -3*time.Hour), member("b", "/w/api", -2*time.Hour), member("c", "/w/infra", -time.Hour),
				{ID: "d", CWD: "/w/site", State: state.Working}, {ID: "e", CWD: "/w/docs", State: state.Working},
			}})
		for range turns {
			s.Turn()
		}
		g := &Game{scene: s, theme: th, sidebar: sidebar, scripted: -1, clicked: -1,
			dispatch:         func(run func()) { run() },
			readConversation: func(string, int, int) (claude.Conversation, error) { return claude.Conversation{}, nil },
		}
		return g
	}
	diving := func(t *testing.T, turns int, sidebar bool) (*Game, city.Rect) {
		t.Helper()
		g := newGame(t, turns, sidebar)
		g.layoutDive(width, height, measure)
		require.True(t, g.scene.FocusGroup("g1"))
		return g, g.layoutDive(width, height, measure)
	}
	bounds := func(g *Game) city.Rect { return city.RectAt(0, 0, width, height) }

	t.Run("when the river is on the right of the screen", func(t *testing.T) {
		g, dock := diving(t, 0, false)

		t.Run("the list should dock on the left", func(t *testing.T) {
			assert.Less(t, dock.Center().X, bounds(g).Center().X)
		})

		t.Run("and the sidebar is open", func(t *testing.T) {
			_, dock := diving(t, 0, true)

			t.Run("the list should start right of the sidebar", func(t *testing.T) {
				assert.GreaterOrEqual(t, dock.Min.X, ui.SidebarWidth(th))
			})
		})
	})

	t.Run("when the heading is turned so the river is on the left", func(t *testing.T) {
		g, dock := diving(t, 2, false)

		t.Run("the list should dock on the right", func(t *testing.T) {
			assert.Greater(t, dock.Center().X, bounds(g).Center().X)
		})
	})

	for turns := range 4 {
		t.Run("at each of the four headings, "+[]string{"north", "east", "south", "west"}[turns], func(t *testing.T) {
			g, dock := diving(t, turns, false)
			river := riverOnScreen(t, g.scene)

			t.Run("the list should not overlap the river", func(t *testing.T) {
				assert.False(t, dock.Overlaps(river), "dock %v, river %v", dock, river)
			})
		})
	}

	for _, turns := range []int{0, 2} {
		t.Run("when the lens panel is laid out beside the list, docked "+[]string{"left", "", "right"}[turns], func(t *testing.T) {
			g, dock := diving(t, turns, false)
			area := g.lensArea(width, height, measure)

			t.Run("it should keep clear of the list", func(t *testing.T) {
				assert.False(t, area.Overlaps(dock), "lens area %v, dock %v", area, dock)
			})
		})
	}

	t.Run("when a row's waited time is long", func(t *testing.T) {
		row := city.DiveRow{Site: "ratelimit-lib", Move: state.MoveTheirs, Waited: 69 * time.Hour}
		for _, inner := range []float64{400, 200, 120} {
			t.Run(fmt.Sprintf("in a list %.0fpx wide", inner), func(t *testing.T) {
				_, move := diveColumns(row, inner, measure)

				t.Run("it should be drawn whole", func(t *testing.T) {
					assert.Contains(t, move, "waited "+format.Age(row.Waited))
				})
			})
		}
	})

	t.Run("when no group is focused", func(t *testing.T) {
		g := newGame(t, 0, false)

		t.Run("it should dock nothing", func(t *testing.T) {
			assert.Zero(t, g.layoutDive(width, height, measure).Area())
		})
	})

	t.Run("when diving", func(t *testing.T) {
		t.Run("and j is pressed", func(t *testing.T) {
			g, _ := diving(t, 0, false)
			pressKey(t, g, ebiten.KeyJ, false)

			t.Run("it should step to the first row", func(t *testing.T) {
				assert.Equal(t, 0, g.scene.DiveStep())
			})
		})

		t.Run("and j then k is pressed", func(t *testing.T) {
			g, _ := diving(t, 0, false)
			pressKey(t, g, ebiten.KeyJ, false)
			pressKey(t, g, ebiten.KeyJ, false)
			pressKey(t, g, ebiten.KeyK, false)

			t.Run("it should step back up a row", func(t *testing.T) {
				assert.Equal(t, 0, g.scene.DiveStep())
			})
		})

		t.Run("and Enter is pressed on a stepped row", func(t *testing.T) {
			g, _ := diving(t, 0, false)
			pressKey(t, g, ebiten.KeyJ, false)
			pressKey(t, g, ebiten.KeyEnter, false)
			r, _ := g.overlays.top().(*reader)

			t.Run("it should open the reading pane for that row's session", func(t *testing.T) {
				require.NotNil(t, r)
				assert.Equal(t, "a", r.id)
			})
		})

		t.Run("and Esc is pressed with a row stepped to", func(t *testing.T) {
			g, _ := diving(t, 0, false)
			pressKey(t, g, ebiten.KeyJ, false)
			pressKey(t, g, ebiten.KeyEscape, false)

			t.Run("it should clear the focus", func(t *testing.T) {
				assert.Empty(t, g.scene.FocusedGroup())
			})

			t.Run("it should clear the selection", func(t *testing.T) {
				assert.Nil(t, g.scene.Selected())
			})
		})

		for _, panel := range []struct {
			name string
			open func(g *Game)
		}{
			{"the finder", func(g *Game) { g.openFinder() }},
			{"New session", func(g *Game) { g.openNewSession() }},
		} {
			t.Run("and "+panel.name+" is open", func(t *testing.T) {
				g, _ := diving(t, 0, false)
				panel.open(g)
				require.NotNil(t, g.overlays.top(), panel.name+" should be open")
				pressKey(t, g, ebiten.KeyJ, false)

				t.Run("j should type, not step", func(t *testing.T) {
					assert.Equal(t, -1, g.scene.DiveStep())
				})
			})
		}
	})
}

// riverOnScreen is the screen box around the river's water.
func riverOnScreen(t *testing.T, s *city.Scene) city.Rect {
	t.Helper()
	from, to, w, ok := s.City().RiverBand()
	require.True(t, ok)
	water := city.Rect{Min: city.Point{X: from.X - w/2, Y: from.Y}, Max: city.Point{X: to.X + w/2, Y: to.Y}}
	cam := s.Camera()
	corners := []city.Point{water.Min, {X: water.Max.X, Y: water.Min.Y}, water.Max, {X: water.Min.X, Y: water.Max.Y}}
	first := cam.WorldToScreen(corners[0])
	out := city.Rect{Min: first, Max: first}
	for _, p := range corners[1:] {
		q := cam.WorldToScreen(p)
		out = out.Union(city.Rect{Min: q, Max: q})
	}
	return out
}
