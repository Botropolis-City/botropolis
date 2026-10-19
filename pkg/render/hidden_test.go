package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 142: while the interface is hidden, a panel's key showed nothing
// and the unseen panel took the keys that followed.
func TestHiddenInterfacePanels(t *testing.T) {
	ticked := func(t *testing.T, before func(g *Game), key ebiten.Key) *Game {
		t.Helper()
		g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1}
		before(g)
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
		return g
	}
	hidden := func(g *Game) { g.hidden = true }
	shown := func(g *Game) bool { return !g.hidden }
	panelOpen := func(g *Game) bool { return g.settingsOpen || g.timeline || g.breakdown }

	cases := []struct {
		name   string
		before func(g *Game)
		key    ebiten.Key
		checks []escapeCheck
	}{
		{"when the interface is hidden, and s is pressed", hidden, ebiten.KeyS,
			[]escapeCheck{
				{"it should show the interface", shown},
				{"it should open settings", func(g *Game) bool { return g.settingsOpen }},
			}},
		{"when the interface is hidden, and t is pressed", hidden, ebiten.KeyT,
			[]escapeCheck{
				{"it should show the interface", shown},
				{"it should open the timeline", func(g *Game) bool { return g.timeline }},
			}},
		{"when the interface is hidden, and x is pressed", hidden, ebiten.KeyX,
			[]escapeCheck{
				{"it should show the interface", shown},
				{"it should open the breakdown", func(g *Game) bool { return g.breakdown }},
			}},
		{"when the interface is hidden, and the plant is clicked",
			func(g *Game) { g.hidden = true; g.clickLandmark(city.LandmarkPlant) }, -1,
			[]escapeCheck{
				{"it should show the interface", shown},
				{"it should open the breakdown", func(g *Game) bool { return g.breakdown }},
			}},
		{"when nothing is open, and h is pressed", func(*Game) {}, ebiten.KeyH,
			[]escapeCheck{
				{"it should hide the interface", func(g *Game) bool { return g.hidden }},
			}},
		{"when settings is open, and h is pressed", func(g *Game) { g.settingsOpen = true }, ebiten.KeyH,
			[]escapeCheck{
				{"it should keep the interface shown", shown},
				{"it should keep settings open", func(g *Game) bool { return g.settingsOpen }},
			}},
		{"when the timeline is open, and h is pressed", func(g *Game) { g.timeline = true }, ebiten.KeyH,
			[]escapeCheck{
				{"it should keep the interface shown", shown},
			}},
		{"when the breakdown is open, and h is pressed", func(g *Game) { g.breakdown = true }, ebiten.KeyH,
			[]escapeCheck{
				{"it should keep the interface shown", shown},
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := ticked(t, tc.before, tc.key)
			checks := append(tc.checks, escapeCheck{
				"it should leave no panel open while the interface is hidden",
				func(g *Game) bool { return !g.hidden || !panelOpen(g) },
			})
			for _, c := range checks {
				t.Run(c.should, func(t *testing.T) {
					assert.True(t, c.holds(g))
				})
			}
		})
	}
}
