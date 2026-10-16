package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 90: the finder finds groups by name too, listed after the
// sessions, and Enter on one focuses it.

func TestFinderGroups(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	member := func(id, cwd string, started time.Duration) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.Working, Group: "g1", StartedAt: now.Add(started), LastActivity: now}
	}
	hit := find.Hit{SessionID: "a", Dir: "/w/lib", State: state.Working, Passage: "kiln headers", Kind: claude.SaidPrompt, At: now}
	// typed opens the finder over a map with one group, types query and
	// delivers hits, then moves the selection down row times.
	typed := func(t *testing.T, query string, hits []find.Hit, row int) (*Game, *recorder) {
		t.Helper()
		actor := &recorder{}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now,
			Groups: []state.Group{{ID: "g1", Name: "PROJ-123 · kiln headers", Short: "PROJ-123", Anchor: "PROJ-123",
				Members: []string{"a", "b"}, Move: state.MoveAgents}},
			Sessions: []state.Session{member("a", "/w/lib", -2*time.Hour), member("b", "/w/api", -time.Hour),
				{ID: "c", CWD: "/w/site", State: state.Working}}})
		g := &Game{scene: s, actor: actor, scripted: -1, clicked: -1}
		pressKey(t, g, ebiten.KeySlash, false)
		f, ok := g.overlays.top().(*finder)
		require.True(t, ok, "the finder should be open")
		f.now = func() time.Time { return time.Time{} }
		for _, r := range query {
			g.scriptedRune = r
			require.NoError(t, g.handleKeys())
		}
		g.scriptedRune = 0
		f.deliver(query, hits)
		require.NoError(t, g.handleKeys())
		for range row {
			pressKey(t, g, ebiten.KeyArrowDown, false)
		}
		return g, actor
	}
	rows := func(g *Game) []ui.FinderResult {
		f, _ := g.overlays.top().(*finder)
		return f.rows(g, now)
	}

	t.Run("when the finder's query matches a group and a session", func(t *testing.T) {
		g, _ := typed(t, "kiln", []find.Hit{hit}, 0)
		got := rows(g)
		require.Len(t, got, 2)

		t.Run("it should list the group after the session hits", func(t *testing.T) {
			assert.Equal(t, []bool{false, true}, []bool{got[0].Group, got[1].Group})
		})

		t.Run("its row should name the group, its route and whose move it is", func(t *testing.T) {
			assert.Equal(t, ui.FinderResult{Group: true, InPrompt: true, Passage: "PROJ-123 · kiln headers",
				Dir: "group  lib → api", Age: "the agents' move"}, got[1])
		})

		t.Run("the status should count the groups and the sessions found", func(t *testing.T) {
			f, _ := g.overlays.top().(*finder)
			assert.Contains(t, f.status(g), "1 session and 1 group found")
		})

		t.Run("and Enter is pressed with nothing moved", func(t *testing.T) {
			g, actor := typed(t, "kiln", []find.Hit{hit}, 0)
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should open the first session hit", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionAttach, SessionID: "a"}}, actor.done)
			})
		})

		t.Run("and down moves to the group and Enter is pressed", func(t *testing.T) {
			g, actor := typed(t, "kiln", []find.Hit{hit}, 1)
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should focus that group", func(t *testing.T) {
				assert.Equal(t, "g1", g.scene.FocusedGroup())
			})

			t.Run("it should close the finder", func(t *testing.T) {
				assert.Nil(t, g.overlays.top())
			})

			t.Run("it should dock the dive", func(t *testing.T) {
				assert.NotEmpty(t, g.scene.Dive())
			})

			t.Run("it should open no session", func(t *testing.T) {
				assert.Empty(t, actor.done)
			})
		})
	})

	t.Run("when the query matches a group and no session", func(t *testing.T) {
		g, _ := typed(t, "PROJ-123", nil, 0)

		t.Run("the group should be the top row", func(t *testing.T) {
			got := rows(g)
			require.NotEmpty(t, got)
			assert.True(t, got[0].Group)
		})

		t.Run("and Enter is pressed", func(t *testing.T) {
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should focus it", func(t *testing.T) {
				assert.Equal(t, "g1", g.scene.FocusedGroup())
			})
		})
	})

	t.Run("when the finder's query matches no group", func(t *testing.T) {
		g, _ := typed(t, "orchard", []find.Hit{hit}, 0)

		t.Run("its rows should be the session hits exactly as before", func(t *testing.T) {
			assert.Equal(t, []ui.FinderResult{finderResult(hit, now)}, rows(g))
		})
	})
}
