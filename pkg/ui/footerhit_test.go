package ui

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bug 43: a footer verb that can only be typed teaches nothing.

func footerFixture() Footer {
	keys := []Key{{Key: "tab", Action: "next needs-you"}, {Key: "v", Action: "views"}, {Key: "q", Action: "quit"}}
	return LayoutFooter(NewTheme(1), 800, 600, "", keys, measureFixed)
}

func TestFooterHit(t *testing.T) {
	f := footerFixture()
	require.Len(t, f.Keys, 3)

	t.Run("when the pointer is on a verb's chip", func(t *testing.T) {
		t.Run("it should find that verb", func(t *testing.T) {
			hit, _ := f.Hit(f.Keys[1].Chip.Center())
			assert.Equal(t, "v", hit.Key)
		})
	})

	t.Run("when the pointer is on the label beside a chip", func(t *testing.T) {
		t.Run("it should still find that verb, not the next one", func(t *testing.T) {
			at := city.Point{X: f.Keys[1].LabelAt.X + 1, Y: f.Keys[1].Chip.Center().Y}
			hit, _ := f.Hit(at)
			assert.Equal(t, "v", hit.Key)
		})
	})

	t.Run("when the pointer is above the key row, over the map", func(t *testing.T) {
		t.Run("it should find nothing, so the click reaches the city", func(t *testing.T) {
			_, ok := f.Hit(city.Point{X: f.Keys[1].Chip.Center().X, Y: f.KeyRow.Min.Y - 10})
			assert.False(t, ok)
		})
	})

	t.Run("when the pointer is left of the first verb", func(t *testing.T) {
		t.Run("it should find nothing rather than the nearest", func(t *testing.T) {
			_, ok := f.Hit(city.Point{X: f.Keys[0].Chip.Min.X - 5, Y: f.Keys[0].Chip.Center().Y})
			assert.False(t, ok)
		})
	})
}
