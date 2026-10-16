package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutCardFace(t *testing.T) {
	th := ui.NewTheme(1)
	bounds := city.RectAt(0, 0, 1000, 800)
	face := city.Card{
		Title:    "use rate-limit headers",
		Kind:     "session in api",
		Move:     "your move",
		Group:    "Rate-limit headers — station 2 of 3",
		Sentence: "Finished Friday 16:40. Tests pass; draft PR #882 is open and waiting for you to mark it ready for review.",
		Numbers:  []city.Number{{Label: "context", Value: "41%"}, {Label: "spent", Value: "$3.20"}, {Label: "idle", Value: "2d"}},
		Actions:  []string{"attach", "stop", "details"},
		Details:  []string{"state    needs-you", "branch   main"},
	}
	laid := func(card city.Card) ui.Card { return ui.LayoutCard(th, card, bounds, measure7) }

	t.Run("when a card leads with a sentence", func(t *testing.T) {
		c := laid(face)

		t.Run("it should put the move at the right of the title row", func(t *testing.T) {
			w, _ := measure7(c.Move.Text, c.Move.Size)
			assert.Equal(t, []float64{c.Title.At.Y, c.Rect.Max.X - 16}, []float64{c.Move.At.Y, c.Move.At.X + w})
		})

		t.Run("it should set the kind and group lines small", func(t *testing.T) {
			assert.Equal(t, []ui.Size{ui.Small, ui.Small}, []ui.Size{c.Kind.Size, c.Group.Size})
		})

		t.Run("it should wrap the sentence at body size", func(t *testing.T) {
			require.Greater(t, len(c.Sentence), 1)
			assert.Equal(t, ui.Body, c.Sentence[len(c.Sentence)-1].Size)
		})

		t.Run("it should set the numbers on one row under it", func(t *testing.T) {
			require.Len(t, c.Numbers, 3)
			last := c.Sentence[len(c.Sentence)-1].At.Y
			assert.True(t, c.Numbers[0].At.Y > last && c.Numbers[0].At.Y == c.Numbers[2].At.Y && c.Numbers[2].At.X > c.Numbers[1].At.X)
		})

		t.Run("it should put the buttons under the numbers", func(t *testing.T) {
			require.NotEmpty(t, c.Buttons)
			assert.Greater(t, c.Buttons[0].Rect.Min.Y, c.Numbers[0].At.Y)
		})
	})

	t.Run("when it is handed more than three numbers", func(t *testing.T) {
		four := face
		four.Numbers = append(append([]city.Number{}, face.Numbers...), city.Number{Label: "age", Value: "5h"})
		c := laid(four)

		t.Run("it should lay three", func(t *testing.T) {
			assert.Len(t, c.Numbers, 3)
		})
	})

	t.Run("when details are open", func(t *testing.T) {
		open := face
		open.Open = true
		c := laid(open)

		t.Run("it should lay each line under the buttons, small", func(t *testing.T) {
			require.Len(t, c.Details, 2)
			assert.True(t, c.Details[0].At.Y >= c.Buttons[0].Rect.Max.Y && c.Details[1].Size == ui.Small && c.Rect.Max.Y > c.Details[1].At.Y)
		})
	})

	t.Run("when the pointer is on a card's button", func(t *testing.T) {
		hinted := face
		hinted.Hint = "Opens this session's terminal."
		c := laid(hinted)

		t.Run("it should show what the action does under the row", func(t *testing.T) {
			assert.True(t, c.Hint.Text == hinted.Hint && c.Hint.At.Y >= c.Buttons[0].Rect.Max.Y && c.Rect.Max.Y > c.Hint.At.Y)
		})
	})

	t.Run("when details are closed", func(t *testing.T) {
		c := laid(face)

		t.Run("it should lay none", func(t *testing.T) {
			assert.Empty(t, c.Details)
		})
	})
}
