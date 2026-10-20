package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutReader(t *testing.T) {
	th := ui.NewTheme(1)
	turns := []ui.ReaderTurn{
		{From: ui.FromYou, Text: "why does the kiln overheat"},
		{From: ui.FromClaude, Text: "The thermocouple is loose."},
		{From: ui.FromTool, Text: "make test-kiln"},
		{From: ui.FromYou, Text: "tighten it"},
	}
	layout := func(width, height float64, in ui.ReaderInput) ui.ReaderPanel {
		return ui.LayoutReader(th, width, height, in, measure7)
	}
	of := func(p ui.ReaderPanel, kind ui.LineKind) []ui.ReaderLine {
		var out []ui.ReaderLine
		for _, l := range append(append([]ui.ReaderLine{}, p.Lead...), p.Lines...) {
			if l.Kind == kind {
				out = append(out, l)
			}
		}
		return out
	}

	t.Run("when the pane is laid out in a 1280×800 window", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Title: "kiln", Turns: turns})

		t.Run("it should centre the panel across the window", func(t *testing.T) {
			assert.Equal(t, 640.0, p.Rect.Center().X)
		})

		t.Run("it should draw with the plates' palette, radius and grid", func(t *testing.T) {
			type look struct {
				Fill   any
				Radius float64
				Pad    float64
			}
			assert.Equal(t, look{th.Palette.Panel, th.Radius(), 2 * th.Grid()}, look{p.Fill, p.Radius, p.Pad})
		})

		t.Run("it should name its keys", func(t *testing.T) {
			for _, key := range []string{"PgUp", "PgDn", "enter to reply", "esc to close"} {
				t.Run(key, func(t *testing.T) {
					assert.Contains(t, p.Keys.Text, key)
				})
			}
		})
	})

	t.Run("when the window is narrower than the panel", func(t *testing.T) {
		p := layout(300, 800, ui.ReaderInput{Title: "kiln", Turns: turns})

		t.Run("it should stay inside the window", func(t *testing.T) {
			assert.True(t, p.Rect.Width() > 0 && p.Rect.Min.X >= 0 && p.Rect.Max.X <= 300, "panel %v", p.Rect)
		})
	})

	t.Run("when there are turns", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Title: "kiln", Turns: turns})
		require.NotEmpty(t, p.Lines)

		t.Run("it should put the newest at the bottom", func(t *testing.T) {
			last := p.Lines[len(p.Lines)-1]
			assert.Equal(t, [2]any{"tighten it", true}, [2]any{last.Text, last.At.Y >= p.Lines[0].At.Y})
		})

		t.Run("it should set your prompts apart from Claude's replies", func(t *testing.T) {
			assert.NotEqual(t, of(p, ui.LineYou)[0].Color, of(p, ui.LineClaude)[0].Color)
		})

		t.Run("it should give each tool call one line", func(t *testing.T) {
			long := layout(1280, 800, ui.ReaderInput{Turns: []ui.ReaderTurn{{From: ui.FromTool, Text: strings.Repeat("make kiln ", 80)}}})
			assert.Len(t, of(long, ui.LineTool), 1)
		})

		t.Run("it should wrap a long reply inside the panel", func(t *testing.T) {
			long := layout(1280, 800, ui.ReaderInput{Turns: []ui.ReaderTurn{{From: ui.FromClaude, Text: strings.Repeat("the kiln is hot ", 60)}}})
			lines := of(long, ui.LineClaude)
			inside := true
			for _, l := range lines {
				w, _ := measure7(l.Text, l.Size)
				inside = inside && l.At.X+w <= long.Rect.Max.X-long.Pad
			}
			assert.True(t, len(lines) > 1 && inside, "%d lines, inside %v", len(lines), inside)
		})
	})

	t.Run("when there are more lines than fit", func(t *testing.T) {
		var many []ui.ReaderTurn
		for i := 1; i <= 200; i++ {
			many = append(many, ui.ReaderTurn{From: ui.FromYou, Text: fmt.Sprintf("turn %d", i)})
		}
		p := layout(1280, 800, ui.ReaderInput{Turns: many})

		t.Run("it should drop the oldest and keep the newest", func(t *testing.T) {
			assert.Equal(t, [2]bool{false, true}, [2]bool{p.Lines[0].Text == "turn 1", p.Lines[len(p.Lines)-1].Text == "turn 200"})
		})

		t.Run("it should say how many turns it shows, newest back", func(t *testing.T) {
			assert.Equal(t, len(p.Lines), p.Shown)
		})

		t.Run("it should keep every line beneath the title and above the keys", func(t *testing.T) {
			_, titleH := measure7("", ui.Title)
			assert.True(t, p.Lines[0].At.Y >= p.Title.At.Y+titleH && p.Lines[len(p.Lines)-1].At.Y < p.Keys.At.Y, "lines %v to %v", p.Lines[0].At, p.Lines[len(p.Lines)-1].At)
		})
	})

	t.Run("when the session is waiting on a question", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Turns: turns, Questions: []ui.ReaderQuestion{
			{Text: "Which kiln should I fix first?", Options: []string{"The glaze kiln", "The bisque kiln"}},
		}})
		require.NotEmpty(t, p.Lead)

		t.Run("and the turns fit beneath it", func(t *testing.T) {
			options := of(p, ui.LineOption)
			_, smallH := measure7("", ui.Small)
			below := options[len(options)-1].At.Y + smallH

			t.Run("it should start them straight under it, leaving no gap", func(t *testing.T) {
				assert.InDelta(t, below+3*th.Grid(), p.Lines[0].At.Y, th.Grid()+smallH, "options end at %v, turns start at %v", below, p.Lines[0].At.Y)
			})
		})

		t.Run("it should lead with the question, above the turns", func(t *testing.T) {
			q := of(p, ui.LineQuestion)
			require.NotEmpty(t, q)
			assert.Equal(t, [2]any{"Which kiln should I fix first?", true}, [2]any{q[0].Text, q[0].At.Y < p.Lines[0].At.Y})
		})

		t.Run("it should list its options beneath it", func(t *testing.T) {
			var options []string
			for _, l := range of(p, ui.LineOption) {
				options = append(options, strings.TrimSpace(strings.TrimPrefix(l.Text, "·")))
			}
			assert.Equal(t, []string{"The glaze kiln", "The bisque kiln"}, options)
		})
	})

	t.Run("when the session is waiting on your approval", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Turns: turns,
			Approval: []string{"rm -rf build/\n  && make kiln"}, Note: "Claude needs your permission to use Bash"})

		t.Run("it should lead with the call it is asking to run, word for word", func(t *testing.T) {
			var call []string
			for _, l := range of(p, ui.LineApproval) {
				call = append(call, l.Text)
			}
			assert.Equal(t, []string{"rm -rf build/", "  && make kiln"}, call)
		})

		t.Run("it should carry the hook's note beneath it", func(t *testing.T) {
			note := of(p, ui.LineNote)
			require.Len(t, note, 1)
			assert.Equal(t, [2]any{"Claude needs your permission to use Bash", true},
				[2]any{note[0].Text, note[0].At.Y > of(p, ui.LineApproval)[1].At.Y})
		})
	})

	t.Run("when the transcript is gone", func(t *testing.T) {
		kept := []ui.ReaderTurn{{From: ui.FromYou, Text: "why does the kiln overheat"}, {From: ui.FromYou, Text: "tighten it"}}
		p := layout(1280, 800, ui.ReaderInput{Turns: kept, Gone: true})

		t.Run("it should say the conversation has been deleted", func(t *testing.T) {
			notes := of(p, ui.LineNote)
			require.NotEmpty(t, notes)
			assert.Contains(t, notes[0].Text, "conversation has been deleted")
		})

		t.Run("it should show the kept prompts as yours", func(t *testing.T) {
			var yours []string
			for _, l := range of(p, ui.LineYou) {
				yours = append(yours, l.Text)
			}
			assert.Equal(t, []string{"why does the kiln overheat", "tighten it"}, yours)
		})

		t.Run("it should name enter as a new session here", func(t *testing.T) {
			assert.Contains(t, p.Keys.Text, "enter for a new session here")
		})
	})

	t.Run("when the transcript is gone, with kept prompts", func(t *testing.T) {
		kept := []ui.ReaderTurn{
			{From: ui.FromYou, Text: "why does the kiln overheat"},
			{From: ui.FromYou, Text: strings.Repeat("convert the celadon glaze recipe ", 12)},
			{From: ui.FromYou, Text: "tighten it"},
		}
		p := layout(1280, 800, ui.ReaderInput{Turns: kept, Gone: true})

		for i := range kept {
			t.Run(fmt.Sprintf("and prompt %d", i), func(t *testing.T) {
				var buttons []ui.ReaderCopy
				for _, c := range p.Copies {
					if c.Turn == i {
						buttons = append(buttons, c)
					}
				}
				var first ui.ReaderLine
				for _, l := range p.Lines {
					if l.Turn == i {
						first = l
						break
					}
				}

				t.Run("it should give the prompt one copy button", func(t *testing.T) {
					assert.Len(t, buttons, 1)
				})

				t.Run("it should put the button beside the prompt's first line, inside the panel", func(t *testing.T) {
					require.Len(t, buttons, 1)
					b := buttons[0].Button.Rect
					w, _ := measure7(first.Text, first.Size)
					assert.True(t, b.Min.Y == first.At.Y && b.Min.X >= first.At.X+w && b.Max.X <= p.Rect.Max.X,
						"button %v, line at %v width %v, panel %v", b, first.At, w, p.Rect)
				})
			})
		}
	})

	t.Run("when the transcript is there", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Turns: turns})

		t.Run("it should give your prompts no copy button", func(t *testing.T) {
			assert.Empty(t, p.Copies)
		})
	})

	t.Run("when Claude's reply is written in Markdown", func(t *testing.T) {
		reply := func(text string) ui.ReaderPanel {
			return layout(1280, 800, ui.ReaderInput{Turns: []ui.ReaderTurn{{From: ui.FromClaude, Text: text}}})
		}

		t.Run("and it holds **bold** and `code`", func(t *testing.T) {
			p := reply("**Step 1: refactor.** `to_html` now delegates to a small `_Parser` class.")

			t.Run("it should lay out no asterisks or backticks", func(t *testing.T) {
				var text []string
				for _, l := range of(p, ui.LineClaude) {
					text = append(text, l.Text)
				}
				assert.False(t, strings.ContainsAny(strings.Join(text, ""), "*`"), "%q", text)
			})
		})

		t.Run("and a list item wraps", func(t *testing.T) {
			lines := of(reply("- "+strings.Repeat("the kiln is hot ", 30)), ui.LineClaude)
			require.True(t, len(lines) > 1, "%d lines", len(lines))
			bullet, _ := measure7("• ", ui.Body)

			t.Run("it should indent its continuation past the bullet", func(t *testing.T) {
				assert.Equal(t, lines[0].At.X+bullet, lines[1].At.X)
			})
		})

		t.Run("and a list item is nested", func(t *testing.T) {
			lines := of(reply("- outer\n  - inner"), ui.LineClaude)
			require.Len(t, lines, 2)

			t.Run("it should indent it further than its parent", func(t *testing.T) {
				assert.Greater(t, lines[1].At.X, lines[0].At.X)
			})
		})

		t.Run("and it holds a code block wider than the pane", func(t *testing.T) {
			lines := of(reply("```\n"+strings.Repeat("x", 400)+"\n```"), ui.LineClaude)

			t.Run("it should clip the code line to one line, not wrap it", func(t *testing.T) {
				assert.Len(t, lines, 1)
			})
		})

		t.Run("and it holds a horizontal rule", func(t *testing.T) {
			p := reply("above\n\n---\n\nbelow")

			t.Run("it should lay out a rule line between the paragraphs", func(t *testing.T) {
				var kinds []ui.LineKind
				for _, l := range p.Lines {
					kinds = append(kinds, l.Kind)
				}
				assert.Equal(t, []ui.LineKind{ui.LineClaude, ui.LineRule, ui.LineClaude}, kinds)
			})
		})

		t.Run("it should carry each line's styled spans beside its text", func(t *testing.T) {
			lines := of(reply("a **b** c"), ui.LineClaude)
			require.Len(t, lines, 1)
			assert.Contains(t, lines[0].Spans, ui.Span{Text: "b", Style: ui.SpanStrong})
		})
	})

	t.Run("when your prompt holds Markdown characters", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Turns: []ui.ReaderTurn{{From: ui.FromYou, Text: "make it **bold**"}}})

		t.Run("it should show them as you typed them", func(t *testing.T) {
			assert.Equal(t, "make it **bold**", of(p, ui.LineYou)[0].Text)
		})
	})

	t.Run("when a tool call holds backticks", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Turns: []ui.ReaderTurn{{From: ui.FromTool, Text: "echo `date`"}}})

		t.Run("it should show them as typed", func(t *testing.T) {
			assert.Equal(t, "echo `date`", of(p, ui.LineTool)[0].Text)
		})
	})

	t.Run("when the start of the conversation is shown", func(t *testing.T) {
		p := layout(1280, 800, ui.ReaderInput{Turns: turns, AtStart: true})

		t.Run("it should say so at the top", func(t *testing.T) {
			assert.Equal(t, [2]any{"start of the conversation", true}, [2]any{p.Start.Text, p.Start.At.Y < p.Lines[0].At.Y})
		})
	})
}
