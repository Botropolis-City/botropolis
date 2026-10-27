package render

import (
	"math"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 280: the clinic's menu lists what needs you, then what is in
// trouble, and a key jumps to each.
func TestClinicMenu(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	needing := func(id, cwd string, since time.Duration) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.NeedsYou, LastActivity: now.Add(-since)}
	}
	erred := state.Session{ID: "erred", CWD: "/w/api", State: state.Working, APIErrors: 1,
		LastErrorAt: now.Add(-4 * time.Minute), LastActivity: now.Add(-4 * time.Minute)}
	newGame := func(sessions ...state.Session) *Game {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: sessions})
		return &Game{scene: s, scripted: -1, clicked: -1}
	}
	everyone := func() *Game {
		return newGame(needing("first", "/w/lib", 3*time.Hour), needing("second", "/w/site", 10*time.Minute), erred,
			state.Session{ID: "fine", CWD: "/w/docs", State: state.Working, LastActivity: now})
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}
	opened := func(t *testing.T) *Game {
		t.Helper()
		g := everyone()
		press(t, g, ebiten.KeyA)
		require.NotNil(t, g.clinicMenu(), "the menu should be open")
		return g
	}
	selected := func(g *Game) string {
		if b := g.scene.Selected(); b != nil {
			return b.Session.ID
		}
		return ""
	}

	t.Run("when the clinic is clicked", func(t *testing.T) {
		g := everyone()
		g.clickLandmark(city.LandmarkClinic)

		t.Run("it should open the menu", func(t *testing.T) {
			assert.NotNil(t, g.clinicMenu())
		})
	})

	t.Run("when a is pressed on the map", func(t *testing.T) {
		g := everyone()
		press(t, g, ebiten.KeyA)

		t.Run("it should open the menu", func(t *testing.T) {
			assert.NotNil(t, g.clinicMenu())
		})
	})

	t.Run("when the menu is open", func(t *testing.T) {
		t.Run("and 2 is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyDigit2)

			t.Run("it should select the second row's session", func(t *testing.T) {
				assert.Equal(t, "second", selected(g))
			})

			t.Run("it should centre the camera on it", func(t *testing.T) {
				b := g.scene.Selected()
				require.NotNil(t, b)
				in, size := g.scene.Insets(), g.scene.Size()
				want := city.Point{X: in.Left + (size.X-in.Left-in.Right)/2, Y: in.Top + (size.Y-in.Top-in.Bottom)/2}
				got := g.scene.Camera().WorldToScreen(b.Rect.Center())
				assert.True(t, math.Abs(got.X-want.X) < 1 && math.Abs(got.Y-want.Y) < 1, "%v, want %v", got, want)
			})

			t.Run("it should close the menu", func(t *testing.T) {
				assert.Nil(t, g.clinicMenu())
			})
		})

		t.Run("and 3 is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyDigit3)

			t.Run("it should select the session in trouble, after those needing you", func(t *testing.T) {
				assert.Equal(t, "erred", selected(g))
			})
		})

		t.Run("and down then enter is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyArrowDown)
			press(t, g, ebiten.KeyEnter)

			t.Run("it should select the second row's session", func(t *testing.T) {
				assert.Equal(t, "second", selected(g))
			})
		})

		t.Run("and a number past its rows is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyDigit9)

			t.Run("it should select nothing", func(t *testing.T) {
				assert.Empty(t, selected(g))
			})

			t.Run("it should stay open", func(t *testing.T) {
				assert.NotNil(t, g.clinicMenu())
			})
		})

		t.Run("and escape is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyEscape)

			t.Run("it should close", func(t *testing.T) {
				assert.Nil(t, g.clinicMenu())
			})
		})
	})

	t.Run("when nothing needs you and nothing is in trouble", func(t *testing.T) {
		t.Run("the menu should say so", func(t *testing.T) {
			assert.Equal(t, "nobody needs you and nothing is in trouble", clinicStatus(nil))
		})
	})

	t.Run("when the clinic's plate is lettered", func(t *testing.T) {
		for _, c := range []struct {
			scenario string
			count    int
			text     string
			tone     ui.Tone
		}{
			{"and three sessions are listed", 3, "clinic 3", ui.ToneNeedsYou},
			{"and none are", 0, "clinic", ui.ToneNone},
		} {
			t.Run(c.scenario, func(t *testing.T) {
				text, tone := clinicPlate(c.count)

				t.Run("it should say the count", func(t *testing.T) {
					assert.Equal(t, c.text, text)
				})

				t.Run("it should take the needs-you colour only when it counts any", func(t *testing.T) {
					assert.Equal(t, c.tone, tone)
				})
			})
		}
	})
}
