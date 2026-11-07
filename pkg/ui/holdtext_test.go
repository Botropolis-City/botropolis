package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351: a cap never covers text. It sits at its control's top
// right where that covers nothing, else at the right end inside the
// control's own row, else at its left.
func TestLayoutCapsKeepsOffText(t *testing.T) {
	th := ui.NewTheme(1)
	grid := th.Grid()
	window := city.RectAt(0, 0, 1000, 800)
	control := ui.Control{Name: "numbers", Rect: city.RectAt(300, 200, 200, 24), Keys: []string{"i"}}
	over := city.RectAt(440, 180, 60, 16)

	t.Run("when the control's top right would cover text and its row's right end is free", func(t *testing.T) {
		held := ui.LayoutCaps(th, []ui.Control{control}, []city.Rect{over, city.RectAt(300, 204, 100, 16)}, window, measure7)
		require.Len(t, held, 1)
		cap := held[0].Caps[0].Rect

		t.Run("it should sit at the right end inside the row", func(t *testing.T) {
			assert.Equal(t, []float64{control.Rect.Max.X - grid/2, control.Rect.Center().Y}, []float64{cap.Max.X, cap.Center().Y})
		})
	})

	t.Run("when its row's right end has text too", func(t *testing.T) {
		held := ui.LayoutCaps(th, []ui.Control{control}, []city.Rect{over, city.RectAt(300, 204, 200, 16)}, window, measure7)
		require.Len(t, held, 1)
		cap := held[0].Caps[0].Rect

		t.Run("it should sit at the control's left, on its row", func(t *testing.T) {
			assert.Equal(t, []float64{control.Rect.Min.X - grid/2, control.Rect.Center().Y}, []float64{cap.Max.X, cap.Center().Y})
		})
	})

	t.Run("when a session's card is laid and its controls are capped", func(t *testing.T) {
		bounds := city.RectAt(0, 0, 1000, 800)
		card := ui.LayoutCard(th, city.Card{
			Title:    "Rename the shared config key",
			Kind:     "session in alpha",
			Move:     "nobody's move",
			Sentence: "Finished Friday 16:40. Tests pass.",
			Numbers:  []city.Number{{Label: "context", Value: "2%"}, {Label: "spent", Value: "$0.03"}, {Label: "idle", Value: "37d2h"}},
			Actions:  []string{"resume", "Session ⌄", "Project ⌄", "Group ⌄"},
			Hints:    []string{"Starts it again.", "Session: resume, read", "Project: reveal folder", "Group: move", "Folds the details out."},
		}, bounds, measure7)
		keys := [][]string{{"enter"}, {"space", "s"}, {"space", "p"}, {"space", "g"}}
		var controls []ui.Control
		for i, b := range card.Buttons {
			controls = append(controls, ui.Control{Name: b.Label.Text, Rect: b.Rect, Keys: keys[i]})
		}
		controls = append(controls, ui.Control{Name: "numbers", Rect: card.NumbersRect, Keys: []string{"i"}})
		text := card.TextRects(measure7)
		require.NotEmpty(t, text)
		held := ui.LayoutCaps(th, controls, text, window, measure7)
		require.Len(t, held, 5)

		t.Run("it should cover none of the card's text with a cap", func(t *testing.T) {
			for _, h := range held {
				for _, c := range h.Caps {
					for _, r := range text {
						assert.False(t, c.Rect.Overlaps(r), "%s's cap %q at %v covers text at %v", h.Name, c.Label.Text, c.Rect, r)
					}
				}
			}
		})
	})
}
