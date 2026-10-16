package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutFinderResults(t *testing.T) {
	th := ui.NewTheme(1)
	inPrompt := ui.FinderResult{
		InPrompt: true, Passage: "why does the kiln overheat, Kiln?", FirstPrompt: "start the kiln",
		Dir: "/src/kiln", Branch: "main", Age: "2h", Title: "Kiln overheating",
	}
	inReply := ui.FinderResult{
		Passage: "The kiln thermocouple is loose.", FirstPrompt: "look at the heat curve",
		Dir: "/src/kiln", Age: "3d", Title: "Kiln overheating",
	}
	layout := func(results []ui.FinderResult, selected int, height float64) ui.FinderPanel {
		return ui.LayoutFinder(th, 1280, height, ui.FinderInput{
			Query: "kiln", Caret: 4, Words: []string{"kiln"}, Results: results, Selected: selected,
		}, measure7)
	}

	t.Run("when there are results", func(t *testing.T) {
		p := layout([]ui.FinderResult{inPrompt, inReply}, 1, 800)
		require.Len(t, p.Rows, 2)
		first, second := p.Rows[0], p.Rows[1]

		t.Run("it should lay one row per session beneath the field", func(t *testing.T) {
			assert.True(t, p.Field.Max.Y <= first.Rect.Min.Y && first.Rect.Max.Y <= second.Rect.Min.Y && second.Rect.Max.Y <= p.Rect.Max.Y,
				"field %v, rows %v %v, panel %v", p.Field, first.Rect, second.Rect, p.Rect)
		})

		t.Run("it should head each row with the prompt that matched", func(t *testing.T) {
			assert.Equal(t, "why does the kiln overheat, Kiln?", first.Head.Text)
		})

		t.Run("and the match was not in a prompt", func(t *testing.T) {
			t.Run("it should head the row with the session's first prompt, never its title", func(t *testing.T) {
				assert.Equal(t, "look at the heat curve", second.Head.Text)
			})

			t.Run("it should show the matching passage beneath", func(t *testing.T) {
				assert.Equal(t, [2]any{"The kiln thermocouple is loose.", true}, [2]any{second.Passage.Text, second.Passage.At.Y > second.Head.At.Y})
			})
		})

		t.Run("it should put the directory, branch and age beneath the head", func(t *testing.T) {
			assert.Equal(t, [2]any{"/src/kiln  main  2h ago", true}, [2]any{first.Where.Text, first.Where.At.Y > first.Head.At.Y})
		})

		t.Run("it should highlight each matched word where it is drawn", func(t *testing.T) {
			var want []ui.Mark
			for _, at := range []int{strings.Index(first.Head.Text, "kiln"), strings.Index(first.Head.Text, "Kiln")} {
				want = append(want, ui.Mark{Line: first.Head, From: first.Head.At.X + float64(at)*7, To: first.Head.At.X + float64(at+4)*7})
			}
			assert.Equal(t, want, first.Marks)
		})

		t.Run("it should mark the selected row", func(t *testing.T) {
			assert.Equal(t, [2]bool{false, true}, [2]bool{first.Selected, second.Selected})
		})
	})

	t.Run("when the match lies past the row's width", func(t *testing.T) {
		long := inPrompt
		long.Passage = strings.Repeat("lorem ipsum ", 30) + "and the kiln overheats"
		p := layout([]ui.FinderResult{long}, 0, 800)
		require.Len(t, p.Rows, 1)

		t.Run("it should show the text around the match", func(t *testing.T) {
			assert.Equal(t, [2]bool{true, true}, [2]bool{strings.HasPrefix(p.Rows[0].Head.Text, "…"), strings.HasSuffix(p.Rows[0].Head.Text, "kiln overheats")}, p.Rows[0].Head.Text)
		})
	})

	t.Run("when there are more rows than fit the window", func(t *testing.T) {
		var many []ui.FinderResult
		for i := range 40 {
			r := inPrompt
			r.Passage = fmt.Sprintf("kiln %d", i)
			many = append(many, r)
		}
		p := layout(many, 30, 600)

		t.Run("it should stop at the window's bottom and keep the selected row in view", func(t *testing.T) {
			var selected bool
			for _, r := range p.Rows {
				selected = selected || (r.Selected && r.Head.Text == "kiln 30")
			}
			assert.True(t, selected && p.Rect.Max.Y <= 600 && len(p.Rows) < 40, "panel %v with %d rows", p.Rect, len(p.Rows))
		})
	})

	t.Run("when a session's transcript is gone", func(t *testing.T) {
		gone := inPrompt
		gone.Gone = true
		p := layout([]ui.FinderResult{gone}, 0, 800)
		require.Len(t, p.Rows, 1)

		t.Run("it should say its conversation has been deleted", func(t *testing.T) {
			assert.Contains(t, p.Rows[0].Where.Text, "conversation deleted")
		})
	})

	t.Run("when a session is packed up", func(t *testing.T) {
		packed := inPrompt
		packed.Packed = true
		p := layout([]ui.FinderResult{packed}, 0, 800)
		require.Len(t, p.Rows, 1)

		t.Run("it should say it is packed up and that enter unpacks it (item 86)", func(t *testing.T) {
			assert.Contains(t, p.Rows[0].Where.Text, "packed up, enter unpacks")
		})
	})

	t.Run("when a row is selected", func(t *testing.T) {
		r := inPrompt
		r.Recap, r.LastPrompt = "Fixing the kiln's thermocouple.", "now tighten the clamp"
		p := layout([]ui.FinderResult{r}, 0, 800)

		t.Run("it should show a preview of its last recap and its next step", func(t *testing.T) {
			var lines []string
			for _, l := range p.Preview {
				lines = append(lines, l.Text)
			}
			assert.Equal(t, []string{"Fixing the kiln's thermocouple.", "left at: now tighten the clamp"}, lines)
		})
	})
}
