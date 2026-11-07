package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 352: no prompt is answered by a key that is not on screen, and
// every button on a dialog shows one. The keys under a button are
// spelled from the keys its handler answers, so this holds both ways.
func TestEveryDialogButtonHasAKeyOnScreen(t *testing.T) {
	const width, height = 1280.0, 800.0
	g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1}
	for _, d := range []*dialog{quitDialog(), demolishDialog("Fix the queue", "a"), closeGroupDialog("PROJ-9", "g1")} {
		t.Run("when the dialog "+d.title+" is laid out", func(t *testing.T) {
			laid := d.layout(g, width, height)
			require.Len(t, laid.Keys, len(d.choices))
			for i, c := range d.choices {
				var shown []string
				for _, cap := range laid.Keys[i] {
					shown = append(shown, cap.Label.Text)
				}
				t.Run("its button "+c.label+" should show at least one key", func(t *testing.T) {
					assert.NotEmpty(t, shown)
				})

				t.Run("every key its button "+c.label+" shows should be one its handler answers", func(t *testing.T) {
					for _, name := range shown {
						key, ok := press(name)
						assert.True(t, ok && c.answers(key), "%q", name)
					}
				})

				t.Run("every key its handler answers for "+c.label+" should be on screen", func(t *testing.T) {
					for _, key := range c.keys {
						if _, alias := keyAliases[key]; alias {
							continue
						}
						assert.Contains(t, shown, keyName(key))
					}
				})
			}
		})
	}
}

// Item 352: demolishing asks in a dialog; d d still demolishes, since the
// first d opens it and the second confirms.
func TestDemolishDialog(t *testing.T) {
	const width, height = 1280.0, 800.0
	at := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	newGame := func(t *testing.T, selected bool) (*Game, *recordingActor) {
		t.Helper()
		actor := &recordingActor{}
		scene := city.NewScene(city.NewLayout())
		scene.Resize(width, height)
		scene.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{{ID: "a", Title: "Fix the queue", CWD: "/src/kiln", State: state.Resting, LastActivity: at}}})
		if selected {
			require.True(t, scene.Select("a"))
		}
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1}
		t.Cleanup(g.settle)
		return g, actor
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
		g.settle()
	}
	button := func(t *testing.T, g *Game, label string) city.Point {
		t.Helper()
		for _, b := range g.dialog().layout(g, width, height).Buttons {
			if b.Label.Text == label {
				return b.Rect.Center()
			}
		}
		require.Failf(t, "no such button", "%q", label)
		return city.Point{}
	}
	demolished := []city.Action{{Kind: city.ActionDemolish, SessionID: "a"}}

	t.Run("when d is pressed with a building selected", func(t *testing.T) {
		g, actor := newGame(t, true)
		press(t, g, ebiten.KeyD)

		t.Run("it should open the dialog naming the session", func(t *testing.T) {
			require.NotNil(t, g.dialog())
			assert.Equal(t, "Demolish Fix the queue?", g.dialog().title)
		})

		t.Run("it should demolish nothing yet", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})

		t.Run("it should show no notice", func(t *testing.T) {
			assert.Empty(t, g.notice.Text(timeNow()))
		})

		for _, c := range []struct {
			name string
			key  ebiten.Key
		}{{"d", ebiten.KeyD}, {"enter", ebiten.KeyEnter}} {
			t.Run("and "+c.name+" is pressed", func(t *testing.T) {
				g, actor := newGame(t, true)
				press(t, g, ebiten.KeyD)
				press(t, g, c.key)

				t.Run("it should demolish the selection", func(t *testing.T) {
					assert.Equal(t, demolished, actor.done)
				})

				t.Run("it should close the dialog", func(t *testing.T) {
					assert.Nil(t, g.dialog())
				})
			})
		}

		t.Run("and esc is pressed", func(t *testing.T) {
			g, actor := newGame(t, true)
			press(t, g, ebiten.KeyD)
			press(t, g, ebiten.KeyEscape)

			t.Run("it should close the dialog", func(t *testing.T) {
				assert.Nil(t, g.dialog())
			})

			t.Run("it should demolish nothing", func(t *testing.T) {
				assert.Empty(t, actor.done)
			})

			t.Run("and d is pressed again", func(t *testing.T) {
				press(t, g, ebiten.KeyD)

				t.Run("it should only open the dialog again", func(t *testing.T) {
					assert.True(t, g.dialog() != nil && len(actor.done) == 0, "dialog %v, done %v", g.dialog(), actor.done)
				})
			})
		})

		t.Run("and Demolish is clicked", func(t *testing.T) {
			g, actor := newGame(t, true)
			press(t, g, ebiten.KeyD)
			g.clickOverlay(button(t, g, "Demolish"))
			require.NoError(t, g.handleKeys())
			g.clicked = -1
			g.settle()

			t.Run("it should demolish the selection", func(t *testing.T) {
				assert.Equal(t, demolished, actor.done)
			})
		})
	})

	t.Run("when d is pressed with nothing selected", func(t *testing.T) {
		g, _ := newGame(t, false)
		press(t, g, ebiten.KeyD)

		t.Run("it should say to select a building first", func(t *testing.T) {
			assert.Equal(t, "select a building first", g.notice.Text(timeNow()))
		})

		t.Run("it should open no dialog", func(t *testing.T) {
			assert.Nil(t, g.dialog())
		})
	})
}
