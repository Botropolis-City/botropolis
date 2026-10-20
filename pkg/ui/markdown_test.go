package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

func TestParseMarkdown(t *testing.T) {
	kinds := func(blocks []ui.MdBlock) []ui.MdKind {
		var out []ui.MdKind
		for _, b := range blocks {
			out = append(out, b.Kind)
		}
		return out
	}
	first := func(t *testing.T, text string) ui.MdBlock {
		blocks := ui.ParseMarkdown(text)
		require.NotEmpty(t, blocks)
		return blocks[0]
	}

	t.Run("when a paragraph holds inline markup", func(t *testing.T) {
		for _, c := range []struct {
			markup, word string
			style        ui.SpanStyle
		}{
			{"**strong**", "strong", ui.SpanStrong},
			{"__strong__", "strong", ui.SpanStrong},
			{"*emphasis*", "emphasis", ui.SpanEmphasis},
			{"_emphasis_", "emphasis", ui.SpanEmphasis},
			{"`code`", "code", ui.SpanCode},
			{"~~struck~~", "struck", ui.SpanPlain},
		} {
			t.Run("and it is "+c.markup, func(t *testing.T) {
				b := first(t, "before "+c.markup+" after")

				t.Run("it should keep the words with no markup characters", func(t *testing.T) {
					assert.Equal(t, "before "+c.word+" after", b.Text())
				})

				t.Run("it should mark the span with its style", func(t *testing.T) {
					var style ui.SpanStyle = -1
					for _, s := range b.Spans {
						if strings.Contains(s.Text, c.word) {
							style = s.Style
						}
					}
					assert.Equal(t, c.style, style)
				})
			})
		}
	})

	t.Run("when underscores sit inside a word", func(t *testing.T) {
		t.Run("it should leave them as written", func(t *testing.T) {
			assert.Equal(t, "call snake_case_name now", first(t, "call snake_case_name now").Text())
		})
	})

	t.Run("when a code span holds markup characters", func(t *testing.T) {
		b := first(t, "see `**not bold**` here")

		t.Run("it should keep them literally, styled as code", func(t *testing.T) {
			assert.Contains(t, b.Spans, ui.Span{Text: "**not bold**", Style: ui.SpanCode})
		})
	})

	t.Run("when the reply has a heading", func(t *testing.T) {
		for _, text := range []string{"# Plan", "## Plan", "Plan\n===="} {
			b := first(t, text)

			t.Run("and it is written "+strings.ReplaceAll(text, "\n", " "), func(t *testing.T) {
				t.Run("it should give it a block of its own without the markers", func(t *testing.T) {
					assert.Equal(t, [2]any{ui.MdHeading, "Plan"}, [2]any{b.Kind, b.Text()})
				})
			})
		}
	})

	t.Run("when the reply has a bulleted list", func(t *testing.T) {
		for _, marker := range []string{"-", "*", "+"} {
			t.Run("and its marker is "+marker, func(t *testing.T) {
				blocks := ui.ParseMarkdown(marker + " one\n" + marker + " two")

				t.Run("it should make each item a list block with a bullet", func(t *testing.T) {
					var got [][3]any
					for _, b := range blocks {
						got = append(got, [3]any{b.Kind, b.Marker, b.Text()})
					}
					assert.Equal(t, [][3]any{{ui.MdItem, "•", "one"}, {ui.MdItem, "•", "two"}}, got)
				})
			})
		}

		t.Run("and an item is nested two spaces deeper", func(t *testing.T) {
			blocks := ui.ParseMarkdown("- outer\n  - inner")

			t.Run("it should give it one more level of depth", func(t *testing.T) {
				var depths []int
				for _, b := range blocks {
					depths = append(depths, b.Depth)
				}
				assert.Equal(t, []int{0, 1}, depths)
			})
		})
	})

	t.Run("when the reply has a numbered list", func(t *testing.T) {
		blocks := ui.ParseMarkdown("3. three\n4. four")

		t.Run("it should keep each item's number", func(t *testing.T) {
			var markers []string
			for _, b := range blocks {
				markers = append(markers, b.Marker)
			}
			assert.Equal(t, []string{"3.", "4."}, markers)
		})
	})

	t.Run("when the reply has a fenced code block", func(t *testing.T) {
		b := first(t, "```go\nx := **1**\n\n  y\n```")

		t.Run("it should keep its lines verbatim, without the fences", func(t *testing.T) {
			assert.Equal(t, [2]any{ui.MdCode, []string{"x := **1**", "", "  y"}}, [2]any{b.Kind, b.Lines})
		})
	})

	t.Run("when the reply has a link", func(t *testing.T) {
		for _, c := range []struct{ name, text, want string }{
			{"and its text differs from its URL", "read [the docs](https://example.com/docs)", "read the docs (https://example.com/docs)"},
			{"and its text is its URL", "read [https://example.com](https://example.com)", "read https://example.com"},
			{"and it is an autolink", "read <https://example.com>", "read https://example.com"},
		} {
			t.Run(c.name, func(t *testing.T) {
				t.Run("it should show the text, and the URL when it differs", func(t *testing.T) {
					assert.Equal(t, c.want, first(t, c.text).Text())
				})
			})
		}
	})

	t.Run("when the reply has a blockquote", func(t *testing.T) {
		b := first(t, "> quoted **text**")

		t.Run("it should make it a quoted block without the marker", func(t *testing.T) {
			assert.Equal(t, [2]any{1, "quoted text"}, [2]any{b.Quote, b.Text()})
		})
	})

	t.Run("when the reply has a horizontal rule", func(t *testing.T) {
		t.Run("it should make it a block of its own", func(t *testing.T) {
			assert.Equal(t, []ui.MdKind{ui.MdParagraph, ui.MdRule, ui.MdParagraph}, kinds(ui.ParseMarkdown("above\n\n---\n\nbelow")))
		})
	})

	t.Run("when the reply has a table", func(t *testing.T) {
		b := first(t, "| name | count |\n|---|---|\n| kiln | 12 |\n| glaze `x` | 3 |")
		require.Len(t, b.Lines, 3)

		t.Run("it should make it a table block", func(t *testing.T) {
			assert.Equal(t, ui.MdTable, b.Kind)
		})

		t.Run("it should leave no pipes or dash rows", func(t *testing.T) {
			assert.False(t, strings.ContainsAny(strings.Join(b.Lines, ""), "|-"), "%q", b.Lines)
		})

		t.Run("it should line its columns up", func(t *testing.T) {
			col := strings.Index(b.Lines[0], "count")
			assert.Equal(t, []int{col, col}, []int{strings.Index(b.Lines[1], "12"), strings.Index(b.Lines[2], "3")}, "%q", b.Lines)
		})
	})

	t.Run("when the reply holds raw HTML", func(t *testing.T) {
		for _, c := range []struct{ name, text, want string }{
			{"and it is inline", "a <b>bold</b> c", "a <b>bold</b> c"},
			{"and it is a block", "<div>\nhi\n</div>", "<div>\nhi\n</div>"},
		} {
			t.Run(c.name, func(t *testing.T) {
				t.Run("it should show it as text", func(t *testing.T) {
					assert.Equal(t, c.want, first(t, c.text).Text())
				})
			})
		}
	})

	t.Run("when the text has no Markdown", func(t *testing.T) {
		blocks := ui.ParseMarkdown("just words here.")

		t.Run("it should come back as one plain paragraph, unchanged", func(t *testing.T) {
			require.Len(t, blocks, 1)
			assert.Equal(t, [2]any{ui.MdParagraph, "just words here."}, [2]any{blocks[0].Kind, blocks[0].Text()})
		})
	})
}
