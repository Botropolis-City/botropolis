package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351: a hovered button's line says what it does and ends with its
// key as a cap, after a dot; the card is as wide as it was, and one size
// whichever button is hovered.
func TestCardHintCap(t *testing.T) {
	th := ui.NewTheme(1)
	bounds := city.RectAt(0, 0, 1000, 800)
	grid := th.Grid()
	pad := 2 * grid
	base := city.Card{
		Title:    "use rate-limit headers",
		Kind:     "session in api",
		Sentence: "Finished Friday 16:40. Tests pass.",
		Actions:  []string{"attach", "Session ⌄", "Project ⌄", "Group ⌄"},
		Hints:    []string{"Opens this session in a terminal.", "Session: attach, stop, read", "Project: reveal folder", "Group: move to group"},
		HintKeys: [][]string{{"enter"}, {"space", "s"}, {"space", "p"}, {"space", "g"}},
	}
	hovering := func(card city.Card, i int) city.Card {
		card.Hint, card.HintKey = card.Hints[i], card.HintKeys[i]
		return card
	}
	laid := func(card city.Card) ui.Card { return ui.LayoutCard(th, card, bounds, measure7) }
	labels := func(caps []ui.KeyCap) []string {
		var out []string
		for _, c := range caps {
			out = append(out, c.Label.Text)
		}
		return out
	}

	t.Run("when a hovered button has a hint and a key", func(t *testing.T) {
		c := laid(hovering(base, 1))
		require.NotEmpty(t, c.Hint)
		last := c.Hint[len(c.Hint)-1]
		lastW, _ := measure7(last.Text, ui.Small)

		t.Run("it should end the line with the key's caps", func(t *testing.T) {
			assert.Equal(t, []string{"space", "s"}, labels(c.HintCaps))
		})

		t.Run("it should set a dot a grid after the line's last word", func(t *testing.T) {
			assert.Equal(t, ui.Text{Text: "·", At: city.Point{X: last.At.X + lastW + grid, Y: last.At.Y}, Size: ui.Small}, c.HintDot)
		})

		t.Run("it should set the caps on the line's row, a grid after the dot", func(t *testing.T) {
			dotW, _ := measure7("·", ui.Small)
			assert.Equal(t, []float64{c.HintDot.At.X + dotW + grid, last.At.Y}, []float64{c.HintCaps[0].Rect.Min.X, c.HintCaps[0].Label.At.Y})
		})

		t.Run("it should keep the caps inside the card", func(t *testing.T) {
			for _, cap := range c.HintCaps {
				assert.Equal(t, c.Rect, c.Rect.Union(cap.Rect), "cap %q at %v", cap.Label.Text, cap.Rect)
			}
		})

		t.Run("it should leave the card as wide as it is with no keys", func(t *testing.T) {
			bare := base
			bare.HintKeys = nil
			bare.Hint = base.Hints[1]
			assert.Equal(t, laid(bare).Rect.Width(), c.Rect.Width())
		})
	})

	t.Run("when the hint's last line has no room for its key", func(t *testing.T) {
		probe := laid(hovering(base, 0))
		inner := probe.Rect.Width() - 2*pad
		full := strings.Repeat("x", int(inner/7))
		card := base
		card.Hints = []string{full, "Session: attach", "Project: reveal", "Group: move"}
		c := laid(hovering(card, 0))
		require.Len(t, c.Hint, 1)
		_, smallH := measure7("", ui.Small)

		t.Run("it should set the caps at the start of the next row", func(t *testing.T) {
			assert.Equal(t, []float64{c.Hint[0].At.X, c.Hint[0].At.Y + smallH}, []float64{c.HintCaps[0].Rect.Min.X, c.HintCaps[0].Label.At.Y})
		})

		t.Run("it should set no dot, since nothing on that row comes before the caps", func(t *testing.T) {
			assert.Empty(t, c.HintDot.Text)
		})

		t.Run("it should keep the caps inside the card", func(t *testing.T) {
			for _, cap := range c.HintCaps {
				assert.Equal(t, c.Rect, c.Rect.Union(cap.Rect), "cap %q at %v", cap.Label.Text, cap.Rect)
			}
		})

		t.Run("and another button is hovered", func(t *testing.T) {
			other := laid(hovering(card, 3))

			t.Run("it should keep the card one size", func(t *testing.T) {
				assert.Equal(t, c.Rect, other.Rect)
			})
		})
	})

	t.Run("when the hovered button has no key", func(t *testing.T) {
		card := hovering(base, 0)
		card.HintKey = nil
		c := laid(card)

		t.Run("it should set no dot", func(t *testing.T) {
			assert.Empty(t, c.HintDot.Text)
		})

		t.Run("it should set no caps", func(t *testing.T) {
			assert.Empty(t, c.HintCaps)
		})
	})

	t.Run("when the card is moved", func(t *testing.T) {
		c := laid(hovering(base, 1))
		moved := c.MoveTo(c.Rect.Min.Add(city.Point{X: 30, Y: 40}))

		t.Run("it should carry the dot", func(t *testing.T) {
			assert.Equal(t, c.HintDot.At.Add(city.Point{X: 30, Y: 40}), moved.HintDot.At)
		})

		t.Run("it should carry the caps", func(t *testing.T) {
			assert.Equal(t, c.HintCaps[0].Rect.Min.Add(city.Point{X: 30, Y: 40}), moved.HintCaps[0].Rect.Min)
		})
	})
}
