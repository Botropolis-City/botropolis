package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 268. A first sighting while something covered the city was
// captioned under the scrim and marked seen, so nobody could read it and
// it never came again. Nothing covered is sighted, and a caption up when
// a cover opens waits for it to close.

func TestCoveredCitySightsNothing(t *testing.T) {
	start := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	const width, height = 800.0, 600.0
	flag := spriteHit{rect: city.RectAt(300, 300, 40, 40), hit: city.Hit{Flag: &city.FlagHit{}}}

	type rig struct {
		g   *Game
		now *time.Time
	}
	newRig := func(t *testing.T) rig {
		t.Helper()
		th := ui.NewTheme(1)
		f, err := newFaces(th)
		require.NoError(t, err)
		now := start
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(width, height)
		return rig{g: &Game{scene: s, theme: th, faces: f, frameHits: []spriteHit{flag}, scripted: -1, clicked: -1}, now: &now}
	}
	captions := func(r rig) []ui.Caption { return r.g.explainCaptions(false, true, width, height) }
	sighted := func(r rig) bool { return r.g.scene.Layout().Sighted[city.KindOf(flag.hit).Name()] }

	t.Run("when nothing covers the city, and a kind is on screen for the first time", func(t *testing.T) {
		r := newRig(t)

		t.Run("it should caption it", func(t *testing.T) {
			assert.Len(t, captions(r), 1)
		})
	})

	for _, c := range []struct {
		name    string
		cover   func(*Game)
		uncover func(*Game)
	}{
		{"the store notice", func(g *Game) { g.overlays.push(&storeNotice{}) }, func(g *Game) { g.overlays.pop() }},
		{"the board", func(g *Game) { g.overlays.push(&board{}) }, func(g *Game) { g.overlays.pop() }},
		{"the close-up", func(g *Game) { g.overlays.push(&closeUp{}) }, func(g *Game) { g.overlays.pop() }},
		{"the quit panel", func(g *Game) { g.openQuit() }, func(g *Game) { g.overlays.pop() }},
		{"the statistics prompt", func(g *Game) { g.overlays.push(&statsPrompt{}) }, func(g *Game) { g.overlays.pop() }},
		{"the reference", func(g *Game) { g.overlays.push(&reference{}) }, func(g *Game) { g.overlays.pop() }},
		{"triage", func(g *Game) { g.overlays.push(&triage{}) }, func(g *Game) { g.overlays.pop() }},
		{"the settings", func(g *Game) { g.settingsOpen = true }, func(g *Game) { g.settingsOpen = false }},
		{"the breakdown", func(g *Game) { g.breakdown = true }, func(g *Game) { g.breakdown = false }},
		{"the timeline", func(g *Game) { g.timeline = true }, func(g *Game) { g.timeline = false }},
		{"the view key", func(g *Game) { g.viewKey = true }, func(g *Game) { g.viewKey = false }},
	} {
		t.Run("when the city is covered by "+c.name+", and a kind is on screen for the first time", func(t *testing.T) {
			r := newRig(t)
			c.cover(r.g)
			got := captions(r)

			t.Run("it should caption nothing", func(t *testing.T) {
				assert.Empty(t, got)
			})

			t.Run("it should not mark the kind sighted", func(t *testing.T) {
				assert.False(t, sighted(r))
			})

			t.Run("and the cover is closed with the kind still on screen", func(t *testing.T) {
				c.uncover(r.g)

				t.Run("it should caption it", func(t *testing.T) {
					assert.Len(t, captions(r), 1)
				})
			})
		})
	}

	t.Run("when a first sighting's caption is up, and the city is covered for longer than the caption stays", func(t *testing.T) {
		r := newRig(t)
		require.Len(t, captions(r), 1)
		*r.now = r.now.Add(time.Second)
		r.g.viewKey = true
		captions(r)
		*r.now = r.now.Add(2 * city.SightingFor)
		captions(r)
		r.g.viewKey = false

		t.Run("and the cover closes, it should caption it again", func(t *testing.T) {
			assert.Len(t, captions(r), 1)
		})
	})
}
