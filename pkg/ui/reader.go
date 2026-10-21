package ui

import (
	"image/color"
	"strings"

	"botropolis.city/botropolis/pkg/city"
)

// ReaderFrom is who a turn in the reading pane is from.
type ReaderFrom int

const (
	FromYou ReaderFrom = iota
	FromClaude
	FromTool
)

// ReaderTurn is one turn of the conversation: a tool call is one line
// already, a prompt or a reply as written.
type ReaderTurn struct {
	From ReaderFrom
	Text string
	// Blocks are a reply already read as Markdown, so a pane laid out
	// every frame does not read it again; without them the layout reads
	// Text itself. Item 182.
	Blocks []MdBlock
}

// ReaderQuestion is a question the session is waiting on you to answer.
type ReaderQuestion struct {
	Text    string
	Options []string
}

// ReaderInput is what the reading pane shows: a page of turns, oldest
// first, and what the session is waiting on you for, which leads. Item 83.
type ReaderInput struct {
	Title     string
	Turns     []ReaderTurn
	AtStart   bool
	Questions []ReaderQuestion
	// Approval is the calls a session needing you is waiting to run, word
	// for word, and Note what Claude Code said when it asked.
	Approval []string
	Note     string
	Err      string
	// Gone says the transcript has been deleted: the turns are the
	// prompts Claude Code's history kept, and Enter starts a new session.
	Gone bool
}

// LineKind is what a line of the reading pane is.
type LineKind int

const (
	LineYou LineKind = iota
	LineClaude
	LineTool
	LineHeading
	LineQuestion
	LineOption
	LineApproval
	LineNote
	LineError
	LineRule
)

// ReaderLine is one line of the reading pane, placed and coloured.
type ReaderLine struct {
	Text  string
	At    city.Point
	Size  Size
	Kind  LineKind
	Color color.NRGBA
	// Turn is the index in the input's turns of the turn the line is part
	// of, for a line of a turn.
	Turn int
	// Runs are the line's text in its styles, each at its offset from
	// the line's start, for a reply read as Markdown. Item 182.
	Runs []Run
	// Gap is the space left above the line: a half-line between a
	// reply's blocks.
	Gap float64
	// CodeBlock says the line is a row of a code block or a table, which
	// is tinted as a block.
	CodeBlock bool
	// Indent is how far right of the pane's text the line starts: a list
	// item's depth, a quote's, and a wrapped item's hang past its bullet.
	Indent float64
}

// ReaderCopy is a kept prompt's copy button, beside its first line, and
// the turn it copies whole. Aria asked for one on each, 2026-10-16.
type ReaderCopy struct {
	Button Button
	Turn   int
}

// readerCopyLabel is the copy button's label.
const readerCopyLabel = "copy"

// ReaderPanel is the reading pane laid out over the city: the title, what
// the session is waiting on, the turns that fit with the newest at the
// bottom, and the keys. Shown is how many turns it shows whole, counted
// back from the newest, which is how far PgUp goes.
type ReaderPanel struct {
	Rect     city.Rect
	Fill     color.NRGBA
	Radius   float64
	Pad      float64
	Title    Text
	Subtitle Text
	Lead     []ReaderLine
	Lines    []ReaderLine
	Start    Text
	Keys     Text
	Shown    int
	// Copies are the kept prompts' copy buttons, when the transcript is gone.
	Copies []ReaderCopy
	// Tints are the faint grounds behind code, a block's across the
	// pane and a span's behind just it, filled in TintColor. Item 182.
	Tints     []city.Rect
	TintColor color.NRGBA
}

// readerGrids is the pane's width when the window has room for it.
const readerGrids = 100

// readerKeys names the pane's keys along its foot.
const readerKeys = "PgUp PgDn older and newer   enter to reply   esc to close"

// readerGoneKeys are the keys when the conversation has been deleted:
// there is nothing to reply to, so Enter starts again where it was.
const readerGoneKeys = "enter for a new session here   esc to close"

// LayoutReader centres the reading pane across a width×height window,
// most of its height tall.
func LayoutReader(th Theme, width, height float64, in ReaderInput, measure Measure) ReaderPanel {
	return LayoutReaderStyled(th, width, height, in, measure, func(s string, size Size, _ SpanStyle) (float64, float64) {
		return measure(s, size)
	})
}

// StyleMeasure is Measure in a span's own face: bold, or monospace for
// code.
type StyleMeasure func(s string, size Size, style SpanStyle) (width, height float64)

// LayoutReaderStyled is LayoutReader with each style of a reply measured
// in its own face, so bold and code wrap and sit at their own widths.
func LayoutReaderStyled(th Theme, width, height float64, in ReaderInput, measure Measure, styled StyleMeasure) ReaderPanel {
	grid := th.Grid()
	pad := 2 * grid
	w := min(readerGrids*grid, width-2*grid)
	p := ReaderPanel{
		Rect:   city.RectAt((width-w)/2, height/12, w, height-height/6),
		Fill:   th.Palette.Panel,
		Radius: th.Radius(),
		Pad:    pad,
	}
	pal := th.Palette
	_, titleH := measure("", Title)
	titleH = subtitled(titleH, measure)
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	inner := w - 2*pad
	x := p.Rect.Min.X + pad
	y := p.Rect.Min.Y + pad
	p.Title = Text{Text: clipTo(in.Title, inner, Title, measure), At: city.Point{X: x, Y: y}, Size: Title}
	p.Subtitle = subtitleUnder(p.Title, PanelReader, measure)
	y += titleH + grid

	add := func(lines *[]ReaderLine, text string, size Size, kind LineKind, c color.NRGBA) {
		_, h := measure("", size)
		*lines = append(*lines, ReaderLine{Text: text, At: city.Point{X: x, Y: y}, Size: size, Kind: kind, Color: c})
		y += h
	}
	if in.Gone {
		add(&p.Lead, clipTo("this conversation has been deleted; these are the prompts Claude Code kept", inner, Small, measure), Small, LineNote, pal.Dim)
		if in.Note != "" {
			add(&p.Lead, clipTo(in.Note, inner, Small, measure), Small, LineNote, pal.Dim)
		}
	}
	if len(in.Approval) > 0 {
		add(&p.Lead, "waiting for your approval to run", Small, LineHeading, pal.NeedsYou)
		for _, call := range in.Approval {
			for _, line := range strings.Split(call, "\n") {
				add(&p.Lead, clipTo(line, inner, Body, measure), Body, LineApproval, pal.Text)
			}
		}
		if in.Note != "" {
			add(&p.Lead, clipTo(in.Note, inner, Small, measure), Small, LineNote, pal.Dim)
		}
	}
	if len(in.Questions) > 0 {
		add(&p.Lead, "asking you", Small, LineHeading, pal.NeedsYou)
		for _, q := range in.Questions {
			for _, line := range wrapPara(q.Text, inner, 1<<10, measure) {
				add(&p.Lead, line, Body, LineQuestion, pal.Text)
			}
			for _, o := range q.Options {
				add(&p.Lead, clipTo("· "+o, inner, Small, measure), Small, LineOption, pal.Dim)
			}
		}
	}
	if len(p.Lead) > 0 {
		y += grid
	}

	keysY := p.Rect.Max.Y - pad - smallH
	keys := readerKeys
	if in.Gone {
		keys = readerGoneKeys
	}
	p.Keys = Text{Text: clipTo(keys, inner, Small, measure), At: city.Point{X: x, Y: keysY}, Size: Small}
	startY := y
	top := y + smallH + grid/2
	bottom := keysY - grid

	if in.Err != "" {
		p.Lines = []ReaderLine{{Text: clipTo(in.Err, inner, Body, measure), At: city.Point{X: x, Y: top}, Size: Body, Kind: LineError, Color: pal.Error}}
		return p
	}

	gap := grid / 2
	var blocks [][]ReaderLine
	room := bottom - top
	copyW, _ := measure(readerCopyLabel, Small)
	copyW += 2 * grid
	textW := inner
	if in.Gone {
		textW -= copyW + grid
	}
	for i := len(in.Turns) - 1; i >= 0; i-- {
		block := turnLines(th, in.Turns[i], textW, measure, styled)
		for j := range block {
			block[j].Turn = i
		}
		h := gap * float64(min(len(blocks), 1))
		for _, l := range block {
			_, lh := measure("", l.Size)
			h += lh + l.Gap
		}
		if h > room {
			if len(blocks) == 0 {
				blocks = append(blocks, block[max(0, len(block)-int(room/bodyH)):])
			}
			break
		}
		room -= h
		blocks = append(blocks, block)
		p.Shown++
	}
	y = bottom
	for _, block := range blocks {
		for j := len(block) - 1; j >= 0; j-- {
			_, lh := measure("", block[j].Size)
			y -= lh
			block[j].At = city.Point{X: x + block[j].Indent, Y: y}
			y -= block[j].Gap
		}
		y -= gap
	}
	for i := len(blocks) - 1; i >= 0; i-- {
		p.Lines = append(p.Lines, blocks[i]...)
	}
	// Turns that do not fill the pane follow straight on from what leads
	// it, rather than leaving a gap above them (Aria, 2026-10-16).
	if len(p.Lines) > 0 {
		if lift := p.Lines[0].At.Y - top; lift > 0 {
			for i := range p.Lines {
				p.Lines[i].At.Y -= lift
			}
		}
	}
	p.TintColor = pal.Hairline
	p.Tints = codeTints(p.Lines, p.Rect.Max.X-pad, grid, measure)
	if in.Gone {
		right := p.Rect.Max.X - pad
		seen := map[int]bool{}
		for _, l := range p.Lines {
			if l.Kind != LineYou || seen[l.Turn] {
				continue
			}
			seen[l.Turn] = true
			p.Copies = append(p.Copies, ReaderCopy{Turn: l.Turn, Button: Button{
				Rect:  city.RectAt(right-copyW, l.At.Y, copyW, bodyH),
				Label: Text{Text: readerCopyLabel, At: city.Point{X: right - copyW + grid, Y: l.At.Y + (bodyH-smallH)/2}, Size: Small},
			}})
		}
	}
	if in.AtStart && p.Shown == len(in.Turns) {
		at := startY
		if len(p.Lines) > 0 {
			at = max(startY, p.Lines[0].At.Y-smallH-gap)
		}
		p.Start = Text{Text: "start of the conversation", At: city.Point{X: x, Y: at}, Size: Small}
	}
	return p
}

// turnLines is a turn as lines: a tool call clipped to one, a prompt or
// a reply wrapped to the pane's width.
func turnLines(th Theme, turn ReaderTurn, width float64, measure Measure, styled StyleMeasure) []ReaderLine {
	pal := th.Palette
	if turn.From == FromTool {
		return []ReaderLine{{Text: clipTo(turn.Text, width, Small, measure), Size: Small, Kind: LineTool, Color: pal.Dim}}
	}
	if turn.From == FromClaude {
		blocks := turn.Blocks
		if blocks == nil {
			blocks = ParseMarkdown(turn.Text)
		}
		return replyLines(th, blocks, width, styled)
	}
	kind, c := LineYou, pal.Accent
	var lines []ReaderLine
	for _, line := range wrapPara(turn.Text, width, 1<<10, measure) {
		lines = append(lines, ReaderLine{Text: line, Size: Body, Kind: kind, Color: c})
	}
	return lines
}
