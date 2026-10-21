package ui

// Run is a span placed on its line: X is its offset from the line's
// start, measured in the faces of the spans before it.
type Run struct {
	Span
	X float64
	W float64
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
	codeBlock := false
	line := func(spans []Span, indent float64, kind LineKind) {
		var runs []Run
		x := 0.0
		for _, s := range spans {
			w, _ := styled(s.Text, Body, s.Style)
			runs = append(runs, Run{Span: s, X: x, W: w})
			x += w
		}
		lines = append(lines, ReaderLine{Text: spanText(spans), Runs: runs, Size: Body, Kind: kind, Color: pal.Text, Indent: indent, Gap: gap, CodeBlock: codeBlock})
		gap = 0
	}
	code := func(s string, size Size) (float64, float64) { return styled(s, size, SpanCode) }
	codeStrong := func(s string, size Size) (float64, float64) { return styled(s, size, SpanCodeStrong) }
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
			codeBlock = true
			for j, l := range b.Lines {
				style, measure := SpanCode, code
				if j < b.Header {
					style, measure = SpanCodeStrong, codeStrong
				}
				l = clipTo(l, room, Body, measure)
				var spans []Span
				if l != "" {
					spans = []Span{{Text: l, Style: style}}
				}
				line(spans, indent, LineClaude)
			}
			codeBlock = false
		case MdItem:
			marker := []Span{{Text: b.Marker + " "}}
			hang := widthOf(marker)
			wrapped := wrapSpans(b.Spans, room-hang, styled)
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
			for _, s := range wrapSpans(spans, room, styled) {
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
// Words are sliced from the spans' own text: a space, a tab and a newline
// are single bytes that never occur inside another character in UTF-8.
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
		text := s.Text
		for i := 0; i < len(text); {
			switch c := text[i]; c {
			case ' ', '\t', '\n':
				flush()
				space = true
				if c == '\n' {
					out = append(out, mdWord{brk: true})
					space = false
				}
				cur.space = space
				i++
			default:
				j := i
				for j < len(text) && text[j] != ' ' && text[j] != '\t' && text[j] != '\n' {
					j++
				}
				if len(cur.pieces) == 0 {
					cur.space = space
				}
				space = false
				cur.pieces = append(cur.pieces, Span{Text: text[i:j], Style: s.Style})
				i = j
			}
		}
	}
	flush()
	return out
}

// wrapSpans is wrapPara for styled text: as many words to a line as fit
// the width in their own faces, a word too long for a line clipped to it.
// It keeps each line's width as it goes, measuring each word once,
// because the pane is laid out every frame. Item 182.
func wrapSpans(spans []Span, width float64, styled StyleMeasure) [][]Span {
	var lines [][]Span
	var cur []Span
	curW := 0.0
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
		cur, curW = nil, 0
	}
	for _, w := range words(spans) {
		if w.brk {
			end()
			continue
		}
		wordW := 0.0
		for _, p := range w.pieces {
			pw, _ := styled(p.Text, Body, p.Style)
			wordW += pw
		}
		if len(cur) > 0 && w.space {
			style := SpanPlain
			if prev := cur[len(cur)-1].Style; prev == w.pieces[0].Style {
				style = prev
			}
			spaceW, _ := styled(" ", Body, style)
			if curW+spaceW+wordW > width {
				end()
			} else {
				add(Span{Text: " ", Style: style})
				curW += spaceW
			}
		} else if len(cur) > 0 && curW+wordW > width {
			end()
		}
		if len(cur) == 0 && wordW > width {
			style := w.pieces[0].Style
			text := spanText(w.pieces)
			cur = []Span{{Text: clipTo(text, width, Body, func(s string, size Size) (float64, float64) {
				return styled(s, size, style)
			}), Style: style}}
			curW = width
			continue
		}
		for _, p := range w.pieces {
			add(p)
		}
		curW += wordW
	}
	end()
	return lines
}
