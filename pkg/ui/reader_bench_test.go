package ui_test

import (
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/ui"
)

// BenchmarkLayoutReader is one frame of the reading pane over a page of
// Markdown replies, as the city lays it out every frame the pane is
// open: with each reply read once per page, as the city hands them over,
// and read again every frame, as it did before slice 4. It reports and
// never gates. Item 182.
func BenchmarkLayoutReader(b *testing.B) {
	reply := strings.Repeat("**Step 1: refactor.** `to_html` now delegates to a small `_Parser` class.\n\n"+
		"- one item\n- two `items`\n\n```go\nx := 1\n```\n\n| a | b |\n|---|---|\n| c | d |\n\n", 3)
	th := ui.NewTheme(1)
	for _, c := range []struct {
		name   string
		blocks []ui.MdBlock
	}{
		{"read once per page", ui.ParseMarkdown(reply)},
		{"read every frame", nil},
	} {
		in := ui.ReaderInput{}
		for range 40 {
			in.Turns = append(in.Turns, ui.ReaderTurn{From: ui.FromClaude, Text: reply, Blocks: c.blocks})
		}
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ui.LayoutReader(th, 1920, 1080, in, measure7)
			}
		})
	}
}
