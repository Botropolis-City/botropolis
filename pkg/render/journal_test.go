package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86, slice 5: coming back to the map after a gap opens the
// journal of where you were and what is next.
func TestComingBack(t *testing.T) {
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	sessions := []state.Session{
		{ID: "a", Title: "Fix the queue", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-60 * time.Hour), Recap: claude.Recap{Text: "Shipped the parser.\nNext: tag v1.4.0"}},
		{ID: "b", Title: "Read the spec", CWD: "/src/orchard", State: state.NeedsYou, Kind: claude.KindBackground, LastActivity: at.Add(-61 * time.Hour)},
	}
	game := func(t *testing.T, away time.Duration) (*Game, *recorder) {
		t.Helper()
		layout := city.NewLayout()
		layout.Seen = at.Add(-away)
		scene := city.NewScene(layout)
		scene.SetClock(func() time.Time { return at }, time.UTC)
		scene.Resize(800, 600)
		actor := &recorder{}
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1, journalHours: 8}
		g.awaySince = layout.Seen
		g.Offer(state.Snapshot{At: at, Sessions: sessions})
		g.takePending()
		return g, actor
	}
	journalOpen := func(g *Game) bool {
		_, ok := g.overlays.top().(*journal)
		return ok
	}

	t.Run("when the map opens after a gap of journal_hours or more", func(t *testing.T) {
		g, _ := game(t, 64*time.Hour)

		t.Run("the journal should open over the map with no input", func(t *testing.T) {
			assert.True(t, journalOpen(g))
		})

		t.Run("it should list the entries most recently touched first", func(t *testing.T) {
			require.True(t, journalOpen(g))
			assert.Equal(t, []string{"Fix the queue", "Read the spec"}, g.overlays.top().(*journal).titles())
		})
	})

	t.Run("when it opens after a shorter gap", func(t *testing.T) {
		g, _ := game(t, 2*time.Hour)

		t.Run("it should not open the journal", func(t *testing.T) {
			assert.False(t, journalOpen(g))
		})
	})

	t.Run("when the map has never been seen before", func(t *testing.T) {
		g, _ := game(t, 0)
		g.awaySince = time.Time{}
		g.overlays = nil
		g.journalChecked = false
		g.Offer(state.Snapshot{At: at, Sessions: sessions})
		g.takePending()

		t.Run("it should not open the journal", func(t *testing.T) {
			assert.False(t, journalOpen(g))
		})
	})

	t.Run("when Esc is pressed", func(t *testing.T) {
		g, _ := game(t, 64*time.Hour)
		pressKey(t, g, ebiten.KeyEscape, false)

		t.Run("it should go to the map", func(t *testing.T) {
			assert.False(t, journalOpen(g))
		})

		for _, c := range []struct {
			name   string
			key    ebiten.Key
			shift  bool
			opened bool
		}{
			{"and Shift+W is pressed", ebiten.KeyW, true, true},
			{"and w is pressed", ebiten.KeyW, false, false},
		} {
			t.Run(c.name, func(t *testing.T) {
				g, _ := game(t, 64*time.Hour)
				pressKey(t, g, ebiten.KeyEscape, false)
				if c.shift {
					pressShift(t, g, c.key)
				} else {
					pressKey(t, g, c.key, false)
				}

				t.Run("the journal should be open only on the journal key", func(t *testing.T) {
					assert.Equal(t, c.opened, journalOpen(g))
				})
			})
		}
	})

	t.Run("when Enter is pressed on an entry", func(t *testing.T) {
		g, _ := game(t, 64*time.Hour)
		pressKey(t, g, ebiten.KeyEnter, false)

		t.Run("it should select its session", func(t *testing.T) {
			require.NotNil(t, g.scene.Selected())
			assert.Equal(t, "a", g.scene.Selected().Session.ID)
		})

		t.Run("it should go to the map", func(t *testing.T) {
			assert.False(t, journalOpen(g))
		})
	})

	t.Run("the since strip", func(t *testing.T) {
		ev := func(kind events.Kind, ago time.Duration) events.Event {
			return events.Event{Kind: kind, At: at.Add(-ago), SessionID: "a"}
		}
		strip := sinceStrip(at.Add(-64*time.Hour), []events.Event{
			ev(events.Ended, time.Hour), ev(events.Merged, 2*time.Hour), ev(events.Error, 3*time.Hour), ev(events.NeedsYou, 4*time.Hour), ev(events.NeedsYou, 5*time.Hour),
		}, 0, time.UTC)

		t.Run("it should say since when", func(t *testing.T) {
			assert.Contains(t, strip, "since Friday 17:00")
		})

		for _, want := range []string{"2 finished", "1 failed", "2 needed you"} {
			t.Run("it should count "+want, func(t *testing.T) {
				assert.Contains(t, strip, want)
			})
		}

		t.Run("it should leave out counts that are zero", func(t *testing.T) {
			assert.Equal(t, "since Friday 17:00", sinceStrip(at.Add(-64*time.Hour), nil, 0, time.UTC))
		})

		t.Run("when a window's reset time falls inside the gap", func(t *testing.T) {
			t.Run("the strip should say it reset", func(t *testing.T) {
				assert.Contains(t, sinceStrip(at.Add(-64*time.Hour), nil, 1, time.UTC), "1 usage window reset")
			})
		})
	})
}

func TestRestoreDesk(t *testing.T) {
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	game := func(t *testing.T, closeUp *state.CloseUp, lastLive []string) (*Game, *recorder) {
		t.Helper()
		layout := city.NewLayout()
		layout.Seen, layout.LastLive = at.Add(-64*time.Hour), lastLive
		scene := city.NewScene(layout)
		scene.SetClock(func() time.Time { return at }, time.UTC)
		scene.Resize(800, 600)
		actor := &recorder{}
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1, journalHours: 8}
		g.awaySince = layout.Seen
		g.lastCloseUp = func() (state.CloseUp, error) {
			if closeUp == nil {
				return state.CloseUp{}, nil
			}
			return *closeUp, nil
		}
		g.Offer(state.Snapshot{At: at, Sessions: []state.Session{
			{ID: "paused", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-62 * time.Hour)},
			{ID: "still", CWD: "/src/kiln", State: state.NeedsYou, LastActivity: at.Add(-61 * time.Hour)},
			{ID: "other", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-90 * time.Hour)},
		}})
		g.takePending()
		return g, actor
	}
	restore := city.Action{Kind: city.ActionRestore, Open: []string{"paused"}}

	t.Run("when R is pressed in the journal", func(t *testing.T) {
		g, actor := game(t, &state.CloseUp{At: at.Add(-64 * time.Hour), Open: []string{"paused", "still"}}, nil)
		pressKey(t, g, ebiten.KeyR, false)
		g.settle()

		t.Run("it should resume in the background each session open at close and not live now", func(t *testing.T) {
			assert.Equal(t, []city.Action{restore}, actor.done)
		})
	})

	t.Run("when the journal is drawn", func(t *testing.T) {
		g, actor := game(t, &state.CloseUp{At: at.Add(-64 * time.Hour), Open: []string{"paused", "still"}}, nil)
		j, ok := g.overlays.top().(*journal)
		require.True(t, ok)
		button := j.restoreButton(g, 800, 600)

		t.Run("it should offer Restore the desk as a button", func(t *testing.T) {
			assert.Equal(t, "Restore the desk", button.Label.Text)
		})

		t.Run("and the button is clicked", func(t *testing.T) {
			g.overlays.top().click(g, button.Rect.Center())
			g.settle()

			t.Run("the button should do what R does", func(t *testing.T) {
				assert.Equal(t, []city.Action{restore}, actor.done)
			})
		})
	})

	t.Run("when nothing was open", func(t *testing.T) {
		g, actor := game(t, &state.CloseUp{At: at.Add(-64 * time.Hour)}, nil)
		pressKey(t, g, ebiten.KeyR, false)

		t.Run("it should ask nothing", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, "nothing was open to restore", g.overlays.top().(*journal).note)
		})
	})

	t.Run("when you did not close up", func(t *testing.T) {
		g, actor := game(t, nil, []string{"paused", "still"})
		pressKey(t, g, ebiten.KeyR, false)
		g.settle()

		t.Run("it should use the sessions the map last saw live", func(t *testing.T) {
			assert.Equal(t, []city.Action{restore}, actor.done)
		})
	})
}

// takePending is Update's snapshot step alone, without the window: the
// offered snapshot reaches the scene, then the journal is considered.
func (g *Game) takePending() {
	g.mu.Lock()
	pending := g.pending
	g.pending = nil
	g.mu.Unlock()
	if pending != nil {
		g.scene.SetSnapshot(*pending)
		g.maybeJournal()
	}
}
