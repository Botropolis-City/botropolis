package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

type escapeCheck struct {
	should string
	holds  func(g *Game) bool
}

// Item 137: Escape's ladder has to hold everything that is open. The
// sidebar and the hidden interface were missing from it, so Escape opened
// settings over the one and underneath the other.
func TestEscapeLadder(t *testing.T) {
	escaped := func(t *testing.T, open func(g *Game)) *Game {
		t.Helper()
		g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1}
		open(g)
		g.scripted = ebiten.KeyEscape
		require.NoError(t, g.handleKeys())
		g.scripted = -1
		return g
	}

	cases := []struct {
		name   string
		open   func(g *Game)
		checks []escapeCheck
	}{
		{"when the sidebar is open, and Escape is pressed",
			func(g *Game) { g.sidebar = true },
			[]escapeCheck{
				{"it should close the sidebar", func(g *Game) bool { return !g.sidebar }},
				{"it should not open settings", func(g *Game) bool { return !g.settingsOpen }},
			}},
		{"when the interface is hidden, and Escape is pressed",
			func(g *Game) { g.hidden = true },
			[]escapeCheck{
				{"it should show the interface", func(g *Game) bool { return !g.hidden }},
				{"it should not open settings", func(g *Game) bool { return !g.settingsOpen }},
			}},
		{"when the interface is hidden, and the sidebar is open, and Escape is pressed",
			func(g *Game) { g.hidden, g.sidebar = true, true },
			[]escapeCheck{
				{"it should show the interface", func(g *Game) bool { return !g.hidden }},
				{"it should leave the sidebar open", func(g *Game) bool { return g.sidebar }},
			}},
		{"when nothing is open, and Escape is pressed",
			func(*Game) {},
			[]escapeCheck{
				{"it should open settings", func(g *Game) bool { return g.settingsOpen }},
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := escaped(t, tc.open)
			for _, c := range tc.checks {
				t.Run(c.should, func(t *testing.T) {
					assert.True(t, c.holds(g))
				})
			}
		})
	}
}
