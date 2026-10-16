package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/find"
)

func TestFinderResults(t *testing.T) {
	t0 := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	hits := []find.Hit{{SessionID: "a"}, {SessionID: "b"}, {SessionID: "c"}}

	// setup opens the finder on a game whose searches run inline and are
	// counted, on a clock the test moves.
	setup := func(t *testing.T) (*Game, *finder, *time.Time, *[]string) {
		t.Helper()
		now := t0
		var asked []string
		g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1}
		g.search = func(q find.Query) ([]find.Hit, error) {
			asked = append(asked, q.Words)
			return hits, nil
		}
		g.dispatch = func(f func()) { f() }
		g.scripted = ebiten.KeySlash
		require.NoError(t, g.handleKeys())
		g.scripted = -1
		f, ok := g.overlays.top().(*finder)
		require.True(t, ok, "the finder should be open")
		f.now = func() time.Time { return now }
		require.Equal(t, findPause, f.pause)
		return g, f, &now, &asked
	}
	tick := func(t *testing.T, g *Game) {
		t.Helper()
		require.NoError(t, g.handleKeys())
	}
	typed := func(t *testing.T, g *Game, s string) {
		t.Helper()
		for _, r := range s {
			g.scriptedRune = r
			tick(t, g)
		}
		g.scriptedRune = 0
	}
	press := func(t *testing.T, g *Game, key ebiten.Key, times int) {
		t.Helper()
		for range times {
			g.scripted = key
			tick(t, g)
		}
		g.scripted = -1
	}

	t.Run("when the query changes", func(t *testing.T) {
		t.Run("and typing pauses", func(t *testing.T) {
			g, _, now, asked := setup(t)
			typed(t, g, "kiln")
			*now = now.Add(findPause)
			tick(t, g)
			tick(t, g)

			t.Run("it should ask for results once", func(t *testing.T) {
				assert.Equal(t, []string{"kiln"}, *asked)
			})
		})

		t.Run("and typing continues", func(t *testing.T) {
			g, _, now, asked := setup(t)
			typed(t, g, "k")
			*now = now.Add(findPause / 2)
			typed(t, g, "i")
			*now = now.Add(findPause / 2)
			tick(t, g)

			t.Run("it should not ask yet", func(t *testing.T) {
				assert.Empty(t, *asked)
			})
		})
	})

	t.Run("when results arrive for a query that has since changed", func(t *testing.T) {
		g, f, _, _ := setup(t)
		typed(t, g, "kil")
		f.deliver("ki", hits)

		t.Run("it should drop them", func(t *testing.T) {
			assert.Empty(t, f.results)
		})
	})

	t.Run("when ↓ or ↑ is pressed", func(t *testing.T) {
		g, f, _, _ := setup(t)
		typed(t, g, "kiln")
		f.deliver("kiln", hits)
		press(t, g, ebiten.KeyArrowDown, 5)
		down := f.selected
		press(t, g, ebiten.KeyArrowUp, 5)

		t.Run("it should move the selection, stopping at either end", func(t *testing.T) {
			assert.Equal(t, [2]int{2, 0}, [2]int{down, f.selected})
		})
	})

	t.Run("when the query is emptied", func(t *testing.T) {
		g, f, _, _ := setup(t)
		typed(t, g, "ki")
		f.deliver("ki", hits)
		press(t, g, ebiten.KeyBackspace, 2)

		t.Run("it should clear the results", func(t *testing.T) {
			assert.Empty(t, f.results)
		})
	})
}
