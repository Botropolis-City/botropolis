package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

func TestOverlayStack(t *testing.T) {
	newGame := func() *Game {
		return &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1}
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted, g.scriptedCtrl = -1, false
	}
	typed := func(t *testing.T, g *Game, s string) {
		t.Helper()
		for _, r := range s {
			g.scriptedRune = r
			require.NoError(t, g.handleKeys())
		}
		g.scriptedRune = 0
	}
	opened := func(t *testing.T) *Game {
		t.Helper()
		g := newGame()
		press(t, g, ebiten.KeySlash)
		require.True(t, g.finding(), "the finder should be open")
		return g
	}

	t.Run("when nothing is open", func(t *testing.T) {
		t.Run("and Ctrl+K is pressed", func(t *testing.T) {
			g := newGame()
			g.scriptedCtrl = true
			press(t, g, ebiten.KeyK)

			t.Run("it should open the finder", func(t *testing.T) {
				assert.True(t, g.finding())
			})
		})

		t.Run("and / is pressed", func(t *testing.T) {
			g := newGame()
			press(t, g, ebiten.KeySlash)

			t.Run("it should open the finder", func(t *testing.T) {
				assert.True(t, g.finding())
			})
		})
	})

	t.Run("when the finder is open", func(t *testing.T) {
		t.Run("and a key the map binds is pressed (t, x, s, Tab)", func(t *testing.T) {
			g := opened(t)
			for _, key := range []ebiten.Key{ebiten.KeyT, ebiten.KeyX, ebiten.KeyS, ebiten.KeyTab} {
				press(t, g, key)
			}

			t.Run("it should not reach the map", func(t *testing.T) {
				assert.Equal(t, [3]bool{}, [3]bool{g.timeline, g.breakdown, g.settingsOpen})
			})
		})

		t.Run("and a character is typed", func(t *testing.T) {
			g := opened(t)
			typed(t, g, "deploy")

			t.Run("it should filter the map behind the panel, as / does today", func(t *testing.T) {
				assert.Equal(t, city.ParseFilter("deploy"), g.scene.Filter())
			})
		})

		t.Run("and Esc is pressed", func(t *testing.T) {
			g := opened(t)
			typed(t, g, "deploy")
			press(t, g, ebiten.KeyEscape)

			t.Run("it should close the finder", func(t *testing.T) {
				assert.False(t, g.finding())
			})

			t.Run("it should clear the filter", func(t *testing.T) {
				assert.True(t, g.scene.Filter().Empty())
			})
		})

		t.Run("and Enter is pressed", func(t *testing.T) {
			g := opened(t)
			typed(t, g, "deploy")
			press(t, g, ebiten.KeyEnter)

			t.Run("it should close the finder and keep the filter", func(t *testing.T) {
				assert.Equal(t, [2]any{false, city.ParseFilter("deploy")}, [2]any{g.finding(), g.scene.Filter()})
			})
		})

		t.Run("and the pointer clicks outside the panel", func(t *testing.T) {
			g := opened(t)
			g.finderRect = city.RectAt(100, 100, 400, 60)
			g.clickOverlay(city.Point{X: 50, Y: 400})

			t.Run("it should close the finder", func(t *testing.T) {
				assert.False(t, g.finding())
			})
		})
	})

	t.Run("when the quit prompt is armed and the finder opens", func(t *testing.T) {
		g := newGame()
		press(t, g, ebiten.KeyQ)
		require.True(t, g.quit.armed(timeNow()), "q should arm the quit prompt")
		press(t, g, ebiten.KeySlash)

		t.Run("it should stand the quit prompt down", func(t *testing.T) {
			assert.False(t, g.quit.armed(timeNow()))
		})
	})
}
