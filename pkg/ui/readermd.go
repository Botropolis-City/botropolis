package ui

import "strings"

// Run is a span placed on its line: X is its offset from the line's
// start, measured in the faces of the spans before it.
type Run struct {
	Span
	X float64
}

// replyLines lays out a reply read as Markdown: prose wrapped by word
// with its styles kept, a list item hung past its bullet, a quote and a
// nested item indented a step a level, code and tables clipped line for
// line rather than wrapped, and a half-line between blocks but not
// between a list's items. Item 182.
func replyLines(th Theme, blocks []MdBlock, width float64, styled StyleMeasure) []ReaderLine {
	pal := th.Palette
	step := 2 * th.Grid()
	_, bodyH := styled("", Body, SpanPlain)
	widthOf := func(spans []Span) float64 {
		w := 0.0
		for _, s := range spans {
			sw, _ := styled(s.Text, Body, s.Style)
			w += sw
		}
		return w
	}
	var lines []ReaderLine
	gap := 0.0
	line := func(spans []Span, indent float64, kind LineKind) {
		var runs []Run
		x := 0.0
		for _, s := range spans {
			runs = append(runs, Run{Span: s, X: x})
			w, _ := styled(s.Text, Body, s.Style)
			x += w
		}
		lines = append(lines, ReaderLine{Text: spanText(spans), Runs: runs, Size: Body, Kind: kind, Color: pal.Text, Indent: indent, Gap: gap})
		gap = 0
	}
	code := func(s string, size Size) (float64, float64) { return styled(s, size, SpanCode) }
	for i, b := range blocks {
		if i > 0 && (b.Kind != MdItem || blocks[i-1].Kind != MdItem) {
			gap = bodyH / 2
		}
		indent := step * float64(b.Depth+b.Quote)
		room := max(width-indent, step)
		switch b.Kind {
		case MdRule:
			line(nil, indent, LineRule)
		case MdCode, MdTable:
			for _, l := range b.Lines {
				l = clipTo(l, room, Body, code)
				var spans []Span
				if l != "" {
					spans = []Span{{Text: l, Style: SpanCode}}
				}
				line(spans, indent, LineClaude)
			}
		case MdItem:
			marker := []Span{{Text: b.Marker + " "}}
			hang := widthOf(marker)
			wrapped := wrapSpans(b.Spans, room-hang, widthOf, styled)
			if len(wrapped) == 0 {
				wrapped = [][]Span{nil}
			}
			for j, spans := range wrapped {
				if j == 0 {
					line(append(marker, spans...), indent, LineClaude)
					continue
				}
				line(spans, indent+hang, LineClaude)
			}
		default:
			spans := b.Spans
			if b.Kind == MdHeading {
				spans = restyle(spans, SpanStrong)
			}
			for _, s := range wrapSpans(spans, room, widthOf, styled) {
				line(s, indent, LineClaude)
			}
		}
	}
	return lines
}

// restyle sets every span but code in one style, as a heading is.
func restyle(spans []Span, style SpanStyle) []Span {
	out := make([]Span, len(spans))
	for i, s := range spans {
		if s.Style != SpanCode {
			s.Style = style
		}
		out[i] = s
	}
	return out
}

// mdWord is a word of styled text: its pieces, and whether a space
// came before it, or a hard break in its place.
type mdWord struct {
	pieces []Span
	space  bool
	brk    bool
}

// words splits spans at their spaces, keeping a word that changes style
// part way through, like **bold**, together. A newline is a hard break.
func words(spans []Span) []mdWord {
	var out []mdWord
	cur := mdWord{}
	space := false
	flush := func() {
		if len(cur.pieces) > 0 {
			out = append(out, cur)
		}
		cur = mdWord{space: space}
	}
	for _, s := range spans {
		var piece strings.Builder
		for _, r := range s.Text {
			switch r {
			case ' ', '\t', '\n':
				if piece.Len() > 0 {
					cur.pieces = append(cur.pieces, Span{Text: piece.String(), Style: s.Style})
					piece.Reset()
				}
				flush()
				space = true
				if r == '\n' {
					out = append(out, mdWord{brk: true})
					space = false
				}
				cur.space = space
			default:
				if len(cur.pieces) == 0 && piece.Len() == 0 {
					cur.space = space
				}
				space = false
				piece.WriteRune(r)
			}
		}
		if piece.Len() > 0 {
			cur.pieces = append(cur.pieces, Span{Text: piece.String(), Style: s.Style})
		}
	}
	flush()
	return out
}

// wrapSpans is wrapPara for styled text: as many words to a line as fit
// the width in their own faces, a word too long for a line clipped to it.
func wrapSpans(spans []Span, width float64, widthOf func([]Span) float64, styled StyleMeasure) [][]Span {
	var lines [][]Span
	var cur []Span
	add := func(line []Span, s Span) []Span {
		if n := len(line); n > 0 && line[n-1].Style == s.Style {
			out := append([]Span{}, line...)
			out[n-1].Text += s.Text
			return out
		}
		return append(append([]Span{}, line...), s)
	}
	end := func() {
		if len(cur) > 0 {
			lines = append(lines, cur)
		}
		cur = nil
	}
	for _, w := range words(spans) {
		if w.brk {
			end()
			continue
		}
		next := cur
		if len(cur) > 0 && w.space {
			style := SpanPlain
			if prev := cur[len(cur)-1].Style; prev == w.pieces[0].Style {
				style = prev
			}
			next = add(next, Span{Text: " ", Style: style})
		}
		for _, p := range w.pieces {
			next = add(next, p)
		}
		if len(cur) > 0 && widthOf(next) > width {
			end()
			next = nil
			for _, p := range w.pieces {
				next = add(next, p)
			}
		}
		if len(cur) == 0 && widthOf(next) > width {
			style := next[0].Style
			next = []Span{{Text: clipTo(spanText(next), width, Body, func(s string, size Size) (float64, float64) {
				return styled(s, size, style)
			}), Style: style}}
		}
		cur = next
	}
	end()
	return lines
}
