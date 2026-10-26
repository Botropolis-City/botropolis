package render

import (
	"errors"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// failing is an actor that cannot do anything, as one with no daemon.
type failing struct{ done []city.Action }

func (f *failing) Do(a city.Action) error {
	f.done = append(f.done, a)
	return errors.New("closing up needs the daemon")
}

// pausing is an actor that records the close-up and then cannot pause bg,
// as the actor reports a pause that failed.
type pausing struct{ done []city.Action }

func (f *pausing) Do(a city.Action) error {
	f.done = append(f.done, a)
	return errors.New("bg: claude stop failed")
}

// Item 86, slice 4: Close up records the desk, and pauses only what you
// choose; everything else is left running.
func TestCloseUpPanel(t *testing.T) {
	at := time.Date(2026, 10, 16, 18, 0, 0, 0, time.UTC)
	live := []state.Session{
		{ID: "bg", Title: "Fix the queue", CWD: "/src/kiln", State: state.NeedsYou, Kind: claude.KindBackground, LastActivity: at},
		{ID: "busy", Title: "Port the loader", CWD: "/src/kiln", State: state.Working, Kind: claude.KindBackground, LastActivity: at.Add(-time.Minute)},
		{ID: "term", Title: "Read the spec", CWD: "/src/orchard", State: state.NeedsYou, Kind: claude.KindInteractive, LastActivity: at.Add(-time.Hour)},
	}
	game := func(t *testing.T, actor Actor, sessions ...state.Session) *Game {
		t.Helper()
		scene := city.NewScene(city.NewLayout())
		scene.Resize(800, 600)
		scene.SetSnapshot(state.Snapshot{At: at, Sessions: append(sessions, state.Session{ID: "rest", CWD: "/src/kiln", State: state.Resting})})
		return &Game{scene: scene, actor: actor, scripted: -1, clicked: -1}
	}
	opened := func(t *testing.T) (*Game, *recorder, *closeUp) {
		t.Helper()
		actor := &recorder{}
		g := game(t, actor, live...)
		pressShift(t, g, ebiten.KeyQ)
		return g, actor, panel(t, g)
	}

	t.Run("when nothing is live", func(t *testing.T) {
		actor := &recorder{}
		g := game(t, actor)
		err := pressingShift(g, ebiten.KeyQ)

		t.Run("Shift+Q should record the close-up without asking", func(t *testing.T) {
			assert.Equal(t, []city.Action{{Kind: city.ActionCloseUp}}, actor.done)
		})

		t.Run("it should open no panel", func(t *testing.T) {
			assert.Nil(t, g.overlays.top())
		})

		t.Run("it should quit the city", func(t *testing.T) {
			assert.Equal(t, ebiten.Termination, err)
		})
	})

	t.Run("when nothing is live and the close-up cannot be recorded", func(t *testing.T) {
		g := game(t, &failing{})
		err := pressingShift(g, ebiten.KeyQ)

		t.Run("it should not quit", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("and the next frame comes", func(t *testing.T) {
			err := pressing(g, -1)

			t.Run("it should still not quit", func(t *testing.T) {
				assert.NoError(t, err)
			})
		})

		t.Run("it should say why in the footer", func(t *testing.T) {
			assert.Equal(t, "closing up needs the daemon", g.notice.Text(timeNow()))
		})
	})

	t.Run("when Shift+Q is pressed with live sessions", func(t *testing.T) {
		_, _, p := opened(t)

		t.Run("the panel should list them", func(t *testing.T) {
			assert.Equal(t, []string{"bg", "busy", "term"}, p.ids())
		})

		t.Run("every row should default to leave running", func(t *testing.T) {
			assert.Empty(t, p.pausedIDs())
		})

		t.Run("a working session's row should say pausing interrupts it", func(t *testing.T) {
			assert.Contains(t, p.line(1), "working — pausing interrupts it")
		})

		t.Run("an interactive session's row should say it runs in a terminal and stays running", func(t *testing.T) {
			assert.Contains(t, p.line(2), "runs in a terminal — stays running")
		})
	})

	t.Run("when p is pressed on a background row", func(t *testing.T) {
		g, _, p := opened(t)
		pressKey(t, g, ebiten.KeyP, false)

		t.Run("it should mark it pause", func(t *testing.T) {
			assert.Equal(t, []string{"bg"}, p.pausedIDs())
		})

		t.Run("and p is pressed again", func(t *testing.T) {
			pressKey(t, g, ebiten.KeyP, false)

			t.Run("it should mark it leave running again", func(t *testing.T) {
				assert.Empty(t, p.pausedIDs())
			})
		})
	})

	t.Run("when p is pressed on an interactive row", func(t *testing.T) {
		g, _, p := opened(t)
		pressKey(t, g, ebiten.KeyArrowDown, false)
		pressKey(t, g, ebiten.KeyArrowDown, false)
		pressKey(t, g, ebiten.KeyP, false)

		t.Run("it should offer no choice and leave it running", func(t *testing.T) {
			assert.Empty(t, p.pausedIDs())
		})
	})

	for _, c := range []struct {
		key  ebiten.Key
		name string
		want []string
	}{
		{ebiten.KeyA, "a", []string{"bg", "busy"}},
		{ebiten.KeyN, "n", nil},
	} {
		t.Run("when "+c.name+" is pressed", func(t *testing.T) {
			g, _, p := opened(t)
			pressKey(t, g, ebiten.KeyP, false)
			pressKey(t, g, c.key, false)

			t.Run("the set-all key should set every row", func(t *testing.T) {
				assert.Equal(t, c.want, p.pausedIDs())
			})
		})
	}

	t.Run("when the panel is laid out", func(t *testing.T) {
		g, _, p := opened(t)
		pressKey(t, g, ebiten.KeyP, false)
		size := g.scene.Size()
		rows := p.layout(g, size.X, size.Y)

		t.Run("a row marked pause should draw its box checked", func(t *testing.T) {
			assert.True(t, rows[0].Checked)
		})

		t.Run("a row left running should draw its box unchecked", func(t *testing.T) {
			assert.False(t, rows[1].Checked)
		})

		t.Run("a terminal row should draw no box", func(t *testing.T) {
			assert.False(t, rows[2].HasBox)
		})

		t.Run("and the box is clicked", func(t *testing.T) {
			p.click(g, rows[1].Box.Center())

			t.Run("a click on the box should toggle it", func(t *testing.T) {
				assert.Equal(t, []string{"bg", "busy"}, p.pausedIDs())
			})
		})

		t.Run("and the pin mark is clicked", func(t *testing.T) {
			p.click(g, rows[2].Pin.Center())

			t.Run("a click on the pin mark should pin it", func(t *testing.T) {
				assert.Equal(t, []string{"term"}, p.pinnedIDs())
			})
		})

		t.Run("and a terminal row's place for a box is clicked", func(t *testing.T) {
			p.click(g, city.Point{X: rows[1].Box.Center().X, Y: rows[2].Pin.Center().Y})

			t.Run("it should still not be marked pause", func(t *testing.T) {
				assert.NotContains(t, p.pausedIDs(), "term")
			})
		})
	})

	t.Run("when Space is pressed on a row", func(t *testing.T) {
		g, _, p := opened(t)
		pressKey(t, g, ebiten.KeySpace, false)

		t.Run("it should pin it", func(t *testing.T) {
			assert.Equal(t, []string{"bg"}, p.pinnedIDs())
		})

		t.Run("and Space is pressed again", func(t *testing.T) {
			pressKey(t, g, ebiten.KeySpace, false)

			t.Run("it should unpin it", func(t *testing.T) {
				assert.Empty(t, p.pinnedIDs())
			})
		})
	})

	t.Run("when Enter is pressed", func(t *testing.T) {
		g, actor, _ := opened(t)
		saved := 0
		g.saveState = func(*city.Layout) { saved++ }
		pressKey(t, g, ebiten.KeyP, false)
		pressKey(t, g, ebiten.KeyArrowDown, false)
		pressKey(t, g, ebiten.KeyArrowDown, false)
		pressKey(t, g, ebiten.KeySpace, false)
		err := pressing(g, ebiten.KeyEnter)

		t.Run("it should ask the actor to close up with the open, pinned and paused sessions", func(t *testing.T) {
			assert.Equal(t, []city.Action{{Kind: city.ActionCloseUp, Open: []string{"bg", "busy", "term"}, Pinned: []string{"term"}, Pause: []string{"bg"}}}, actor.done)
		})

		t.Run("it should close the panel", func(t *testing.T) {
			assert.Nil(t, g.overlays.top())
		})

		t.Run("it should quit the city", func(t *testing.T) {
			assert.Equal(t, ebiten.Termination, err)
		})

		t.Run("it should save the layout as quitting does", func(t *testing.T) {
			assert.Equal(t, 1, saved)
		})
	})

	for _, c := range []struct {
		name  string
		actor Actor
		why   string
	}{
		{"when the actor cannot record the close-up", &failing{}, "closing up needs the daemon"},
		{"when a pause fails", &pausing{}, "bg: claude stop failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := game(t, c.actor, live...)
			pressShift(t, g, ebiten.KeyQ)
			err := pressing(g, ebiten.KeyEnter)
			p, open := g.overlays.top().(*closeUp)

			t.Run("the panel should stay open", func(t *testing.T) {
				assert.True(t, open)
			})

			t.Run("it should say why", func(t *testing.T) {
				require.True(t, open)
				assert.Equal(t, c.why, p.err)
			})

			t.Run("it should not quit", func(t *testing.T) {
				assert.NoError(t, err)
			})

			t.Run("and another key is pressed", func(t *testing.T) {
				err := pressing(g, ebiten.KeyArrowDown)

				t.Run("it should still not quit", func(t *testing.T) {
					assert.NoError(t, err)
				})
			})
		})
	}

	t.Run("when Esc is pressed", func(t *testing.T) {
		g, actor, _ := opened(t)
		pressKey(t, g, ebiten.KeyEscape, false)

		t.Run("it should close the panel", func(t *testing.T) {
			assert.Nil(t, g.overlays.top())
		})

		t.Run("it should ask nothing", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})
	})

	t.Run("when the quit panel is shown", func(t *testing.T) {
		g := game(t, &recorder{}, live...)
		pressKey(t, g, ebiten.KeyQ, false)
		size := g.scene.Size()
		var closeUpButton city.Point
		for _, b := range g.quitPanel().layout(g, size.X, size.Y).Buttons {
			if b.Label.Text == "Close up…" {
				closeUpButton = b.Rect.Center()
			}
		}

		t.Run("and Close up… is clicked", func(t *testing.T) {
			g.clickOverlay(closeUpButton)
			require.NoError(t, g.handleKeys())
			g.clicked = -1

			t.Run("it should open the close-up panel", func(t *testing.T) {
				panel(t, g)
			})

			t.Run("and Enter is pressed", func(t *testing.T) {
				err := pressing(g, ebiten.KeyEnter)

				t.Run("it should quit the city", func(t *testing.T) {
					assert.Equal(t, ebiten.Termination, err)
				})
			})
		})
	})
}

func panel(t *testing.T, g *Game) *closeUp {
	t.Helper()
	p, ok := g.overlays.top().(*closeUp)
	require.True(t, ok, "the close-up panel should be open")
	return p
}

// pressShift presses a key with Shift held, for a tick.
// pressing is a frame's keys with key pressed, or none for -1, and what
// the frame returned: ebiten.Termination when it quits.
func pressing(g *Game, key ebiten.Key) error {
	g.scripted = key
	defer func() { g.scripted = -1 }()
	return g.handleKeys()
}

func pressingShift(g *Game, key ebiten.Key) error {
	g.scriptedShift = true
	defer func() { g.scriptedShift = false }()
	return pressing(g, key)
}

func pressShift(t *testing.T, g *Game, key ebiten.Key) {
	t.Helper()
	g.scripted, g.scriptedShift = key, true
	require.NoError(t, g.handleKeys())
	g.scripted, g.scriptedShift = -1, false
}
