package ui_test

import (
	"strings"
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

	t.Run("when a card offers several actions, each with a hint", func(t *testing.T) {
		hinted := face
		hinted.Hints = []string{
			"Opens this session's terminal.",
			"Stops this session. Its conversation is kept, and you can resume it.",
			"Deletes this session and its worktree, and takes it off the map. Its conversation stays in Claude Code's files.",
		}
		longest := hinted.Hints[2]
		resting := laid(hinted)

		t.Run("and the pointer is on no button", func(t *testing.T) {
			for _, hint := range hinted.Hints {
				over := hinted
				over.Hint = hint

				t.Run("it should lay the same rect as with the hint "+hint, func(t *testing.T) {
					assert.Equal(t, resting.Rect, laid(over).Rect)
				})
			}
		})

		t.Run("and the pointer is on the button with the longest hint", func(t *testing.T) {
			over := hinted
			over.Hint = longest
			c := laid(over)
			require.NotEmpty(t, c.Hint)
			last := c.Hint[len(c.Hint)-1]

			t.Run("it should wrap the hint to more than one line", func(t *testing.T) {
				assert.Greater(t, len(c.Hint), 1)
			})

			t.Run("it should keep every hint line inside the card's width", func(t *testing.T) {
				for _, line := range c.Hint {
					w, _ := measure7(line.Text, line.Size)
					assert.LessOrEqual(t, line.At.X+w, c.Rect.Max.X-16)
				}
			})

			t.Run("it should keep the last hint line above the card's bottom pad", func(t *testing.T) {
				assert.LessOrEqual(t, last.At.Y+16, c.Rect.Max.Y-16)
			})

			t.Run("it should lose none of the hint", func(t *testing.T) {
				var words []string
				for _, line := range c.Hint {
					words = append(words, line.Text)
				}
				assert.Equal(t, longest, strings.Join(words, " "))
			})
		})

		t.Run("and the pointer is on a button", func(t *testing.T) {
			over := hinted
			over.Hint = hinted.Hints[0]
			c := laid(over)

			t.Run("it should show what the action does under the row", func(t *testing.T) {
				require.Len(t, c.Hint, 1)
				assert.True(t, c.Hint[0].Text == over.Hint && c.Hint[0].At.Y >= c.Buttons[0].Rect.Max.Y && c.Rect.Max.Y > c.Hint[0].At.Y)
			})
		})
	})

	t.Run("when a card offers no actions", func(t *testing.T) {
		bare := face
		bare.Actions = nil

		t.Run("and it is handed a hint", func(t *testing.T) {
			hinted := bare
			hinted.Hints = []string{"Opens this session's terminal."}
			hinted.Hint = hinted.Hints[0]

			t.Run("it should lay no hint row", func(t *testing.T) {
				assert.Equal(t, laid(bare).Rect, laid(hinted).Rect)
			})
		})
	})

	t.Run("when details are closed", func(t *testing.T) {
		c := laid(face)

		t.Run("it should lay none", func(t *testing.T) {
			assert.Empty(t, c.Details)
		})
	})
}
