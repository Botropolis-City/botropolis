package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 264. A notice dims the city behind it, and the city behind it went
// on taking the hover: the pointer resting on the notice raised the card of
// whatever stood under it. Nothing a scrim covers is hovered.

func TestCoveredCityTakesNoHover(t *testing.T) {
	now := time.Date(2026, 10, 26, 11, 0, 0, 0, time.UTC)
	const width, height = 1280.0, 800.0

	newGame := func(t *testing.T) (*Game, city.Point) {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(width, height)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{
			{ID: "a", CWD: "/w/tidepool", State: state.Working, StartedAt: now.Add(-time.Hour), LastActivity: now},
		}})
		var ground city.Point
		found := false
		for _, d := range s.City().Districts {
			for y := d.Rect.Min.Y; y <= d.Rect.Max.Y && !found; y += 2 {
				for x := d.Rect.Min.X; x <= d.Rect.Max.X && !found; x += 2 {
					hit := s.City().At(city.Point{X: x, Y: y})
					if hit.District == d && hit.Building == nil {
						ground, found = s.Camera().WorldToScreen(city.Point{X: x, Y: y}), true
					}
				}
			}
		}
		require.True(t, found, "no open ground in any district")
		return &Game{scene: s, hover: ground, hoverSet: true, scripted: -1, clicked: -1}, ground
	}

	t.Run("when nothing covers the city, and the pointer rests on a district", func(t *testing.T) {
		g, _ := newGame(t)
		g.notePointer()

		t.Run("it should hover the district", func(t *testing.T) {
			assert.NotNil(t, g.scene.Hover().District)
		})
	})

	for _, c := range []struct {
		name  string
		cover func(*Game)
	}{
		{"the store notice", func(g *Game) { g.overlays.push(&storeNotice{}) }},
		{"the board", func(g *Game) { g.overlays.push(&board{}) }},
		{"the close-up", func(g *Game) { g.overlays.push(&closeUp{}) }},
		{"the quit panel", func(g *Game) { g.openQuit() }},
		{"the statistics prompt", func(g *Game) { g.overlays.push(&statsPrompt{}) }},
		{"the reference", func(g *Game) { g.overlays.push(&reference{}) }},
		{"triage", func(g *Game) { g.overlays.push(&triage{}) }},
		{"the settings", func(g *Game) { g.settingsOpen = true }},
		{"the breakdown", func(g *Game) { g.breakdown = true }},
		{"the timeline", func(g *Game) { g.timeline = true }},
		{"the view key", func(g *Game) { g.viewKey = true }},
	} {
		t.Run("when the city is covered by "+c.name+", and the pointer rests on a district", func(t *testing.T) {
			g, _ := newGame(t)
			c.cover(g)
			g.notePointer()

			t.Run("it should hover nothing", func(t *testing.T) {
				assert.Equal(t, city.Hit{}, g.scene.Hover())
			})
		})
	}

	t.Run("when the store notice is closed again, and the pointer still rests on the district", func(t *testing.T) {
		g, _ := newGame(t)
		g.overlays.push(&storeNotice{})
		g.notePointer()
		g.overlays.pop()
		g.notePointer()

		t.Run("it should hover the district again", func(t *testing.T) {
			assert.NotNil(t, g.scene.Hover().District)
		})
	})
}
