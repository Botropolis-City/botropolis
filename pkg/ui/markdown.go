package ui

import (
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// SpanStyle is how a run of a reply's text is set.
type SpanStyle int

const (
	SpanPlain SpanStyle = iota
	SpanStrong
	SpanEmphasis
	SpanCode
	SpanCodeStrong
)

// Span is a run of text in one style.
type Span struct {
	Text  string
	Style SpanStyle
}

// MdKind is what a block of a reply is.
type MdKind int

const (
	MdParagraph MdKind = iota
	MdHeading
	MdItem
	MdCode
	MdRule
	MdTable
)

// MdBlock is one block of a reply read as Markdown: a paragraph, heading
// or list item as styled spans, a code block or table as lines set
// verbatim. Depth is how deep in lists it sits, Quote how deep in
// blockquotes. Item 182.
type MdBlock struct {
	Kind   MdKind
	Spans  []Span
	Lines  []string
	Marker string
	Depth  int
	Quote  int
	Level  int
	// Header is how many of a table's lines are its header.
	Header int
}

// Text is the block's words without their markup.
func (b MdBlock) Text() string {
	var s strings.Builder
	for _, sp := range b.Spans {
		s.WriteString(sp.Text)
	}
	return s.String()
}

var markdown = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))

// ParseMarkdown reads a reply as Markdown, a block at a time. Raw HTML is
// kept as the text it was written as, never interpreted.
func ParseMarkdown(src string) []MdBlock {
	source := []byte(src)
	doc := markdown.Parser().Parse(text.NewReader(source))
	r := mdReader{source: source, depth: -1}
	r.blocks(doc)
	return r.out
}

type mdReader struct {
	source []byte
	out    []MdBlock
	depth  int
	quote  int
}

func (r *mdReader) add(b MdBlock) {
	b.Depth, b.Quote = max(r.depth, 0), r.quote
	r.out = append(r.out, b)
}

func (r *mdReader) blocks(parent ast.Node) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		r.block(n)
	}
}

func (r *mdReader) block(n ast.Node) {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		r.add(MdBlock{Kind: MdParagraph, Spans: r.inlines(n)})
	case *ast.Heading:
		r.add(MdBlock{Kind: MdHeading, Spans: r.inlines(n), Level: n.Level})
	case *ast.ThematicBreak:
		r.add(MdBlock{Kind: MdRule})
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		r.add(MdBlock{Kind: MdCode, Lines: r.lines(n.Lines())})
	case *ast.HTMLBlock:
		lines := r.lines(n.Lines())
		if n.HasClosure() {
			lines = append(lines, strings.TrimRight(string(n.ClosureLine.Value(r.source)), "\r\n"))
		}
		r.add(MdBlock{Kind: MdParagraph, Spans: []Span{{Text: strings.Join(lines, "\n")}}})
	case *ast.Blockquote:
		r.quote++
		r.blocks(n)
		r.quote--
	case *ast.List:
		r.depth++
		number := n.Start
		for item := n.FirstChild(); item != nil; item = item.NextSibling() {
			marker := "•"
			if n.IsOrdered() {
				marker = strconv.Itoa(number) + string(n.Marker)
				number++
			}
			at := len(r.out)
			r.blocks(item)
			if at < len(r.out) && r.out[at].Kind == MdParagraph && r.out[at].Depth == r.depth {
				r.out[at].Kind, r.out[at].Marker = MdItem, marker
			} else {
				r.out = append(r.out[:at], append([]MdBlock{{Kind: MdItem, Marker: marker, Depth: r.depth, Quote: r.quote}}, r.out[at:]...)...)
			}
		}
		r.depth--
	case *east.Table:
		header := 0
		if _, ok := n.FirstChild().(*east.TableHeader); ok {
			header = 1
		}
		r.add(MdBlock{Kind: MdTable, Lines: r.table(n), Header: header})
	default:
		r.blocks(n)
	}
}

func (r *mdReader) lines(segs *text.Segments) []string {
	var out []string
	for i := 0; i < segs.Len(); i++ {
		s := segs.At(i)
		out = append(out, strings.TrimRight(string(s.Value(r.source)), "\r\n"))
	}
	return out
}

// table sets each row's cells in columns as wide as their widest cell,
// two spaces apart, with no pipes and no delimiter row.
func (r *mdReader) table(t *east.Table) []string {
	var rows [][]string
	widths := []int{}
	for row := t.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			var s strings.Builder
			for _, sp := range r.inlines(cell) {
				s.WriteString(sp.Text)
			}
			c := s.String()
			if len(widths) <= len(cells) {
				widths = append(widths, 0)
			}
			widths[len(cells)] = max(widths[len(cells)], len([]rune(c)))
			cells = append(cells, c)
		}
		rows = append(rows, cells)
	}
	var out []string
	for _, cells := range rows {
		var line strings.Builder
		for i, c := range cells {
			if i > 0 {
				line.WriteString("  ")
			}
			line.WriteString(c)
			if i < len(cells)-1 {
				line.WriteString(strings.Repeat(" ", widths[i]-len([]rune(c))))
			}
		}
		out = append(out, strings.TrimRight(line.String(), " "))
	}
	return out
}

func (r *mdReader) inlines(parent ast.Node) []Span {
	var spans []Span
	emit := func(s string, style SpanStyle) {
		if s == "" {
			return
		}
		if n := len(spans); n > 0 && spans[n-1].Style == style {
			spans[n-1].Text += s
			return
		}
		spans = append(spans, Span{Text: s, Style: style})
	}
	var walk func(n ast.Node, style SpanStyle)
	walk = func(n ast.Node, style SpanStyle) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch c := c.(type) {
			case *ast.Text:
				emit(string(c.Value(r.source)), style)
				switch {
				case c.HardLineBreak():
					emit("\n", style)
				case c.SoftLineBreak():
					emit(" ", style)
				}
			case *ast.String:
				emit(string(c.Value), style)
			case *ast.CodeSpan:
				var s strings.Builder
				for t := c.FirstChild(); t != nil; t = t.NextSibling() {
					if t, ok := t.(*ast.Text); ok {
						s.Write(t.Value(r.source))
					}
				}
				emit(s.String(), SpanCode)
			case *ast.Emphasis:
				next := SpanEmphasis
				if c.Level >= 2 || style == SpanStrong {
					next = SpanStrong
				}
				walk(c, next)
			case *ast.Link:
				at := len(spanText(spans))
				walk(c, style)
				label := spanText(spans)[at:]
				if dest := string(c.Destination); dest != "" && dest != label {
					emit(" ("+dest+")", SpanPlain)
				}
			case *ast.Image:
				walk(c, style)
				emit(" ("+string(c.Destination)+")", SpanPlain)
			case *ast.AutoLink:
				emit(string(c.Label(r.source)), style)
			case *ast.RawHTML:
				for i := 0; i < c.Segments.Len(); i++ {
					s := c.Segments.At(i)
					emit(string(s.Value(r.source)), style)
				}
			default:
				walk(c, style)
			}
		}
	}
	walk(parent, SpanPlain)
	if n := len(spans); n > 0 {
		spans[n-1].Text = strings.TrimRight(spans[n-1].Text, " ")
	}
	return spans
}

func spanText(spans []Span) string {
	var s strings.Builder
	for _, sp := range spans {
		s.WriteString(sp.Text)
	}
	return s.String()
}
