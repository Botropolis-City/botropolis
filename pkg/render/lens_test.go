package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func measure7(s string, _ ui.Size) (float64, float64) {
	return float64(len([]rune(s))) * 7, 16
}

// Item 88: the lens panel and the finder share the top-left corner.
func TestLensPanelAndFinder(t *testing.T) {
	th := ui.NewTheme(1)
	panel := city.LensPanel{Name: "Capacity", About: "How full each session's context is.", Aggregate: "3 sessions over 80%", Do: "Select a full one."}

	t.Run("when the finder has left its query under the strip", func(t *testing.T) {
		plate := ui.LayoutPlate(th, "/ rate limit   2 of 6", city.Point{X: 16, Y: 40}, ui.Body, measure7).Rect
		laid := ui.LayoutLensPanel(th, panel, lensBounds(th, 32, 1200, 800, false, plate), measure7)

		t.Run("it should not overlap the query", func(t *testing.T) {
			assert.False(t, laid.Rect.Overlaps(plate))
		})
	})

	t.Run("when the finder is open", func(t *testing.T) {
		g := &Game{}
		g.overlays.push(&lensPanel{})
		g.overlays.push(&finder{})

		t.Run("it should not draw the lens panel", func(t *testing.T) {
			assert.False(t, g.lensPanelShown())
		})
	})

	t.Run("when e is held with the finder open", func(t *testing.T) {
		g := &Game{scriptedHeld: map[ebiten.Key]bool{ebiten.KeyE: true}}
		g.overlays.push(&finder{})

		t.Run("it should not explain, because e is a letter there", func(t *testing.T) {
			assert.False(t, g.explainHeld())
		})
	})

	t.Run("when e is held over the map", func(t *testing.T) {
		g := &Game{scriptedHeld: map[ebiten.Key]bool{ebiten.KeyE: true}}

		t.Run("it should explain", func(t *testing.T) {
			assert.True(t, g.explainHeld())
		})
	})

	t.Run("when nothing is over the map", func(t *testing.T) {
		g := &Game{}
		g.overlays.push(&lensPanel{})

		t.Run("it should draw the lens panel", func(t *testing.T) {
			assert.True(t, g.lensPanelShown())
		})
	})
}
