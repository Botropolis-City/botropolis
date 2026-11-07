package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351: a key that takes several presses is one cap per press, set
// left to right.
func TestLayoutKeyCaps(t *testing.T) {
	th := ui.NewTheme(1)
	at := city.Point{X: 100, Y: 50}

	t.Run("when caps are laid for a sequence", func(t *testing.T) {
		caps := ui.LayoutKeyCaps(th, []string{"space", "s", "p"}, at, measure7)
		require.Len(t, caps, 3)

		t.Run("it should lay one cap per step with each step's key", func(t *testing.T) {
			var labels []string
			for _, c := range caps {
				labels = append(labels, c.Label.Text)
			}
			assert.Equal(t, []string{"space", "s", "p"}, labels)
		})

		t.Run("it should start the first where it is put", func(t *testing.T) {
			assert.Equal(t, at, caps[0].Rect.Min)
		})

		t.Run("it should set each next cap half a grid after the last", func(t *testing.T) {
			for i := 1; i < len(caps); i++ {
				assert.Equal(t, caps[i-1].Rect.Max.X+4, caps[i].Rect.Min.X, "step %d", i)
			}
		})

		t.Run("it should say how wide the row of caps is", func(t *testing.T) {
			assert.Equal(t, caps[2].Rect.Max.X-caps[0].Rect.Min.X, ui.KeyCapsWidth(th, []string{"space", "s", "p"}, measure7))
		})
	})

	t.Run("when caps are laid for no keys", func(t *testing.T) {
		t.Run("it should lay none", func(t *testing.T) {
			assert.Empty(t, ui.LayoutKeyCaps(th, nil, at, measure7))
		})

		t.Run("it should be no width", func(t *testing.T) {
			assert.Zero(t, ui.KeyCapsWidth(th, nil, measure7))
		})
	})
}
