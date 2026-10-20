package ui

import "strings"

// replyLines lays out a reply read as Markdown: prose wrapped by word
// with its styles kept, a list item hung past its bullet, a quote and a
// nested item indented a step a level, and code and tables clipped line
// for line rather than wrapped. Item 182.
func replyLines(th Theme, blocks []MdBlock, width float64, measure Measure) []ReaderLine {
	pal := th.Palette
	step := 2 * th.Grid()
	var lines []ReaderLine
	line := func(spans []Span, indent float64, kind LineKind) {
		lines = append(lines, ReaderLine{Text: spanText(spans), Spans: spans, Size: Body, Kind: kind, Color: pal.Text, Indent: indent})
	}
	for _, b := range blocks {
		indent := step * float64(b.Depth+b.Quote)
		room := max(width-indent, step)
		switch b.Kind {
		case MdRule:
			line(nil, indent, LineRule)
		case MdCode, MdTable:
			for _, l := range b.Lines {
				l = clipTo(l, room, Body, measure)
				var spans []Span
				if l != "" {
					spans = []Span{{Text: l, Style: SpanCode}}
				}
				line(spans, indent, LineClaude)
			}
		case MdItem:
			marker := b.Marker + " "
			hang, _ := measure(marker, Body)
			for i, spans := range wrapSpans(b.Spans, room-hang, measure) {
				if i == 0 {
					line(append([]Span{{Text: marker}}, spans...), indent, LineClaude)
					continue
				}
				line(spans, indent+hang, LineClaude)
			}
			if len(b.Spans) == 0 {
				line([]Span{{Text: marker}}, indent, LineClaude)
			}
		default:
			spans := b.Spans
			if b.Kind == MdHeading {
				spans = restyle(spans, SpanStrong)
			}
			for _, s := range wrapSpans(spans, room, measure) {
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
// came before it.
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
// the width, a word too long for a line clipped to it.
func wrapSpans(spans []Span, width float64, measure Measure) [][]Span {
	var lines [][]Span
	var cur []Span
	add := func(s Span) {
		if n := len(cur); n > 0 && cur[n-1].Style == s.Style {
			cur[n-1].Text += s.Text
			return
		}
		cur = append(cur, s)
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
		next := spanText(cur)
		if next != "" && w.space {
			next += " "
		}
		for _, p := range w.pieces {
			next += p.Text
		}
		if fits, _ := measure(next, Body); fits > width && len(cur) > 0 {
			end()
		} else if len(cur) > 0 && w.space {
			style := SpanPlain
			if prev := cur[len(cur)-1].Style; prev == w.pieces[0].Style {
				style = prev
			}
			add(Span{Text: " ", Style: style})
		}
		for _, p := range w.pieces {
			add(p)
		}
		if len(cur) == len(w.pieces) {
			if wide, _ := measure(spanText(cur), Body); wide > width {
				cur = []Span{{Text: clipTo(spanText(cur), width, Body, measure), Style: cur[0].Style}}
			}
		}
	}
	end()
	return lines
}
