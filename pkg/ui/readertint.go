package ui

import (
	"strings"

	"botropolis.city/botropolis/pkg/city"
)

// codeTints are the grounds behind a reply's code, from its placed
// lines: one across the pane's text width for each run of code-block
// rows, padded into the half-line around the block, and one behind each
// inline code run, a little wider than it, on its own line's rows. A
// span that wraps is a run on each line, so each piece is tinted on its
// own row. Item 182.
func codeTints(lines []ReaderLine, right, grid float64, measure Measure) []city.Rect {
	var tints []city.Rect
	padX, spanPad := grid/2, grid/4
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		_, lh := measure("", l.Size)
		if l.CodeBlock {
			last := i
			for last+1 < len(lines) && lines[last+1].CodeBlock && lines[last+1].Turn == l.Turn && lines[last+1].Gap == 0 {
				last++
			}
			padY := lh / 4
			top := l.At.Y - padY
			_, lastH := measure("", lines[last].Size)
			bottom := lines[last].At.Y + lastH + padY
			left := l.At.X - padX
			tints = append(tints, city.RectAt(left, top, right+padX-left, bottom-top))
			i = last
			continue
		}
		for _, r := range l.Runs {
			if r.Style == SpanCode && strings.TrimSpace(r.Text) != "" {
				tints = append(tints, city.RectAt(l.At.X+r.X-spanPad, l.At.Y, r.W+2*spanPad, lh))
			}
		}
	}
	return tints
}
