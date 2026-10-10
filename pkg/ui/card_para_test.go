package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutCardParagraph(t *testing.T) {
	th := ui.NewTheme(1)
	bounds := city.RectAt(0, 0, 1600, 900)
	words := strings.TrimSpace(strings.Repeat("word ", 40))

	t.Run("when a card has no paragraph", func(t *testing.T) {
		c := ui.LayoutCard(th, city.Card{Title: "t", Lines: []string{"one"}}, bounds, measure7)

		t.Run("it should lay no paragraph lines", func(t *testing.T) {
			assert.Empty(t, c.Para)
		})
	})

	t.Run("when a card has a paragraph wider than the card", func(t *testing.T) {
		c := ui.LayoutCard(th, city.Card{Title: "t", Lines: []string{"one"}, Para: words}, bounds, measure7)
		texts := make([]string, 0, len(c.Para))
		for _, line := range c.Para {
			texts = append(texts, line.Text)
		}

		t.Run("it should wrap at word boundaries without losing a word", func(t *testing.T) {
			assert.Equal(t, words, strings.Join(texts, " "))
		})

		t.Run("it should keep every line inside the paragraph measure", func(t *testing.T) {
			for _, text := range texts {
				w, _ := measure7(text, ui.Body)
				assert.LessOrEqual(t, w, ui.ParaWidth(th))
			}
		})

		t.Run("it should widen the card to the widest wrapped line", func(t *testing.T) {
			widest := 0.0
			for _, text := range texts {
				w, _ := measure7(text, ui.Body)
				widest = max(widest, w)
			}
			assert.Equal(t, widest+2*16, c.Rect.Width())
		})

		t.Run("it should start a blank line under the lines", func(t *testing.T) {
			require.NotEmpty(t, c.Para)
			assert.Equal(t, 2*16.0, c.Para[0].At.Y-c.Lines[0].At.Y)
		})

		t.Run("it should hold the paragraph inside the card", func(t *testing.T) {
			last := c.Para[len(c.Para)-1]
			assert.LessOrEqual(t, last.At.Y+16+16, c.Rect.Max.Y)
		})
	})

	t.Run("when a paragraph is longer than the window is tall", func(t *testing.T) {
		short := city.RectAt(0, 0, 1600, 200)
		c := ui.LayoutCard(th, city.Card{Title: "t", Lines: []string{"one"}, Para: strings.Repeat(words+" ", 10)}, short, measure7)

		t.Run("it should end the last line it keeps with an ellipsis", func(t *testing.T) {
			require.NotEmpty(t, c.Para)
			assert.True(t, strings.HasSuffix(c.Para[len(c.Para)-1].Text, "…"))
		})

		t.Run("it should stay inside the window", func(t *testing.T) {
			assert.LessOrEqual(t, c.Rect.Height(), short.Height()-2*16)
		})
	})

	t.Run("when a paragraph has a word longer than the measure", func(t *testing.T) {
		long := strings.Repeat("x", 200)
		c := ui.LayoutCard(th, city.Card{Title: "t", Para: long}, bounds, measure7)

		t.Run("it should clip the word rather than overflow", func(t *testing.T) {
			require.Len(t, c.Para, 1)
			w, _ := measure7(c.Para[0].Text, ui.Body)
			assert.LessOrEqual(t, w, ui.ParaWidth(th))
		})
	})
}
