package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350: the card's face is four buttons, which fit one row in a
// window 960 px wide, and its numbers row is the click that folds the
// details.
func TestLayoutCardFaceButtons(t *testing.T) {
	th := ui.NewTheme(1)
	face := city.Card{
		Title:    "use rate-limit headers",
		Kind:     "session in api",
		Group:    "Rate-limit headers — station 2 of 3",
		Sentence: "Finished Friday 16:40. Tests pass; draft PR #882 is open and waiting for you to mark it ready for review.",
		Numbers:  []city.Number{{Label: "context", Value: "41%"}, {Label: "spent", Value: "$3.20"}, {Label: "idle", Value: "2d"}},
		Actions:  []string{"attach", "Session ▾", "Project ▾", "Group ▾"},
	}

	t.Run("when a card has a face of four buttons in a 960 px window", func(t *testing.T) {
		c := ui.LayoutCard(th, face, city.RectAt(0, 0, 960, 600), measure7)
		require.Len(t, c.Buttons, 4)

		t.Run("it should lay them on one row", func(t *testing.T) {
			assert.Equal(t, c.Buttons[0].Rect.Min.Y, c.Buttons[3].Rect.Min.Y)
		})

		t.Run("it should be no wider than its paragraph and padding", func(t *testing.T) {
			assert.LessOrEqual(t, c.Rect.Width(), ui.ParaWidth(th)+4*th.Grid())
		})
	})

	t.Run("when a card has a numbers row", func(t *testing.T) {
		c := ui.LayoutCard(th, face, city.RectAt(0, 0, 960, 600), measure7).MoveTo(city.Point{X: 100, Y: 100})
		last := c.Numbers[2]
		w, _ := measure7(last.Text, last.Size)

		t.Run("it should give the row a rect that spans every number", func(t *testing.T) {
			assert.True(t, c.NumbersRect.Contains(c.Numbers[0].At) && c.NumbersRect.Contains(city.Point{X: last.At.X + w - 1, Y: last.At.Y}))
		})

		t.Run("it should keep the rect clear of the buttons", func(t *testing.T) {
			assert.LessOrEqual(t, c.NumbersRect.Max.Y, c.Buttons[0].Rect.Min.Y)
		})
	})

	t.Run("when a card has no numbers", func(t *testing.T) {
		bare := face
		bare.Numbers = nil
		c := ui.LayoutCard(th, bare, city.RectAt(0, 0, 960, 600), measure7)

		t.Run("it should give the row no rect", func(t *testing.T) {
			assert.Zero(t, c.NumbersRect.Area())
		})
	})
}
