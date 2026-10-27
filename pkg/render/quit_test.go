package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 292. The quit prompt was a line in the footer, with close up
// beside it; it is a panel in the middle of the window, over the dimmed
// city, and it stays until it is answered.
func TestQuitPanel(t *testing.T) {
	const width, height = 1280.0, 800.0
	at := time.Date(2026, 10, 26, 18, 0, 0, 0, time.UTC)

	type rig struct {
		g     *Game
		actor *recorder
		saved int
	}
	newRig := func(t *testing.T, sessions ...state.Session) *rig {
		t.Helper()
		r := &rig{actor: &recorder{}}
		scene := city.NewScene(city.NewLayout())
		scene.Resize(width, height)
		scene.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
		r.g = &Game{scene: scene, actor: r.actor, scripted: -1, clicked: -1, saveState: func(*city.Layout) { r.saved++ }}
		return r
	}
	press := func(g *Game, key ebiten.Key) error {
		g.scripted = key
		defer func() { g.scripted = -1 }()
		return g.handleKeys()
	}
	opened := func(t *testing.T, sessions ...state.Session) *rig {
		t.Helper()
		r := newRig(t, sessions...)
		require.NoError(t, press(r.g, ebiten.KeyQ))
		return r
	}
	quitOpen := func(g *Game) bool {
		_, ok := g.overlays.top().(*quitPanel)
		return ok
	}
	button := func(t *testing.T, g *Game, label string) city.Point {
		t.Helper()
		for _, b := range g.quitPanel().layout(g, width, height).Buttons {
			if b.Label.Text == label {
				return b.Rect.Center()
			}
		}
		require.Failf(t, "no such button", "%q", label)
		return city.Point{}
	}
	// clickThen clicks a point and runs the key path, as one update does:
	// a button stands in for its key.
	clickThen := func(g *Game, at city.Point) error {
		g.clickOverlay(at)
		defer func() { g.clicked = -1 }()
		return g.handleKeys()
	}
	live := state.Session{ID: "bg", Title: "Fix the queue", CWD: "/src/kiln", State: state.NeedsYou, Kind: claude.KindBackground, LastActivity: at}

	t.Run("when q is pressed on the map", func(t *testing.T) {
		r := opened(t)

		t.Run("it should open the quit panel", func(t *testing.T) {
			assert.True(t, quitOpen(r.g))
		})

		t.Run("it should centre the panel in the window", func(t *testing.T) {
			assert.Equal(t, city.Point{X: width / 2, Y: height / 2}, r.g.quitPanel().layout(r.g, width, height).Plate.Center())
		})

		t.Run("it should put nothing in the footer", func(t *testing.T) {
			assert.Empty(t, r.g.notice.Text(timeNow()))
		})

		t.Run("it should offer Quit, Close up… and Cancel, in that order", func(t *testing.T) {
			var labels []string
			for _, b := range r.g.quitPanel().layout(r.g, width, height).Buttons {
				labels = append(labels, b.Label.Text)
			}
			assert.Equal(t, []string{"Quit", "Close up…", "Cancel"}, labels)
		})

		t.Run("and the loop runs on for a minute and a thousand updates", func(t *testing.T) {
			r := opened(t)
			start := timeNow()
			restore := timeNow
			defer func() { timeNow = restore }()
			timeNow = func() time.Time { return start.Add(time.Minute) }
			for range 1000 {
				require.NoError(t, r.g.handleKeys())
			}

			t.Run("it should still be open", func(t *testing.T) {
				assert.True(t, quitOpen(r.g))
			})
		})
	})

	for _, c := range []struct {
		name string
		key  ebiten.Key
	}{{"y", ebiten.KeyY}, {"enter", ebiten.KeyEnter}, {"keypad enter", ebiten.KeyKPEnter}} {
		t.Run("when "+c.name+" is pressed with the panel open", func(t *testing.T) {
			r := opened(t)
			err := press(r.g, c.key)

			t.Run("it should end the game", func(t *testing.T) {
				assert.Equal(t, ebiten.Termination, err)
			})

			t.Run("it should save the layout first", func(t *testing.T) {
				assert.Equal(t, 1, r.saved)
			})
		})
	}

	t.Run("when the Quit button is clicked", func(t *testing.T) {
		r := opened(t)
		err := clickThen(r.g, button(t, r.g, "Quit"))

		t.Run("it should end the game", func(t *testing.T) {
			assert.Equal(t, ebiten.Termination, err)
		})
	})

	for _, c := range []struct {
		name   string
		answer func(t *testing.T, g *Game) error
	}{
		{"c is pressed", func(_ *testing.T, g *Game) error { return press(g, ebiten.KeyC) }},
		{"Close up… is clicked", func(t *testing.T, g *Game) error { return clickThen(g, button(t, g, "Close up…")) }},
	} {
		t.Run("when "+c.name+" with sessions live", func(t *testing.T) {
			r := opened(t, live)
			require.NoError(t, c.answer(t, r.g))

			t.Run("it should close the quit panel and open the close-up panel", func(t *testing.T) {
				_, ok := r.g.overlays.top().(*closeUp)
				assert.Equal(t, []any{1, true}, []any{len(r.g.overlays), ok})
			})
		})

		t.Run("when "+c.name+" with nothing live", func(t *testing.T) {
			r := opened(t)
			require.NoError(t, c.answer(t, r.g))
			r.g.settle()
			err := press(r.g, -1)

			t.Run("it should record the close-up", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionCloseUp}}, r.actor.done)
			})

			t.Run("it should end the game, as closing up does (item 291)", func(t *testing.T) {
				assert.Equal(t, ebiten.Termination, err)
			})
		})
	}

	for _, c := range []struct {
		name   string
		answer func(t *testing.T, g *Game) error
	}{
		{"n is pressed", func(_ *testing.T, g *Game) error { return press(g, ebiten.KeyN) }},
		{"escape is pressed", func(_ *testing.T, g *Game) error { return press(g, ebiten.KeyEscape) }},
		{"q is pressed again", func(_ *testing.T, g *Game) error { return press(g, ebiten.KeyQ) }},
		{"Cancel is clicked", func(t *testing.T, g *Game) error { return clickThen(g, button(t, g, "Cancel")) }},
	} {
		t.Run("when "+c.name+" with the panel open", func(t *testing.T) {
			r := opened(t)
			err := c.answer(t, r.g)

			t.Run("it should not end the game", func(t *testing.T) {
				assert.NoError(t, err)
			})

			t.Run("it should close the panel", func(t *testing.T) {
				assert.False(t, quitOpen(r.g))
			})
		})
	}

	for _, c := range []struct {
		name string
		key  ebiten.Key
	}{{"x", ebiten.KeyX}, {"t", ebiten.KeyT}, {"s", ebiten.KeyS}, {"/", ebiten.KeySlash}} {
		t.Run("when "+c.name+" is pressed with the panel open", func(t *testing.T) {
			r := opened(t)
			require.NoError(t, press(r.g, c.key))

			t.Run("it should stay open", func(t *testing.T) {
				assert.True(t, quitOpen(r.g))
			})

			t.Run("it should open nothing else", func(t *testing.T) {
				assert.Equal(t, []any{1, false, false, false}, []any{len(r.g.overlays), r.g.settingsOpen, r.g.timeline, r.g.breakdown})
			})
		})
	}

	t.Run("when y is pressed with no panel open", func(t *testing.T) {
		r := newRig(t)

		t.Run("it should not end the game", func(t *testing.T) {
			assert.NoError(t, press(r.g, ebiten.KeyY))
		})
	})

	findChord := defaultChords().Open
	t.Run("when "+findChord.String()+" is pressed with the panel open", func(t *testing.T) {
		r := opened(t)
		pressChord(t, r.g, findChord)

		t.Run("it should stand the panel down and open the finder", func(t *testing.T) {
			assert.Equal(t, []any{1, true}, []any{len(r.g.overlays), r.g.finding()})
		})
	})

	t.Run("when a click lands on the panel but on none of its buttons", func(t *testing.T) {
		r := opened(t)
		plate := r.g.quitPanel().layout(r.g, width, height).Plate
		require.NoError(t, clickThen(r.g, city.Point{X: plate.Min.X + 2, Y: plate.Min.Y + 2}))

		t.Run("it should stay open", func(t *testing.T) {
			assert.True(t, quitOpen(r.g))
		})
	})

	t.Run("when the panel is opened by one call, as a close request will", func(t *testing.T) {
		r := newRig(t)
		r.g.openQuit()
		r.g.openQuit()

		t.Run("it should open one panel", func(t *testing.T) {
			assert.Equal(t, []any{1, true}, []any{len(r.g.overlays), quitOpen(r.g)})
		})
	})

	t.Run("when the panel is opened by that call over another panel", func(t *testing.T) {
		r := newRig(t)
		r.g.openQuit()
		r.g.overlays.push(&board{})
		r.g.openQuit()

		t.Run("it should be the one on top, and the only quit panel", func(t *testing.T) {
			assert.Equal(t, []any{2, true}, []any{len(r.g.overlays), quitOpen(r.g)})
		})
	})
}
