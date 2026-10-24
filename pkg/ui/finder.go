package ui

import (
	"image/color"
	"strings"
	"unicode/utf8"

	"botropolis.city/botropolis/pkg/city"
)

// FinderInput is what the finder panel shows: the query as typed, with
// any composition already in it, the caret as a rune index into it, and
// a line under the field saying what the query found.
type FinderInput struct {
	Query  string
	Caret  int
	Status string
	// Words are the query's words, highlighted wherever a row shows them.
	Words    []string
	Results  []FinderResult
	Selected int
}

// FinderResult is one session the finder found, as the panel shows it.
// InPrompt says the passage is a prompt of yours; when it is not, the row
// is headed by the session's first prompt and the passage goes beneath.
// The title is carried and never shown: a row is headed by your words.
type FinderResult struct {
	// Packed is a session packed up into the depot (item 86).
	Packed      bool
	InPrompt    bool
	Passage     string
	FirstPrompt string
	Title       string
	Dir         string
	Branch      string
	Age         string
	Gone        bool
	Recap       string
	LastPrompt  string
	// Group is a group found by its name (item 90): Dir is then its
	// route and Age whose move it is.
	Group bool
}

// Mark is a highlighted span of a line of text, in screen x.
type Mark struct {
	Line     Text
	From, To float64
}

// FinderRow is one result laid out: its head, where and when beneath,
// and the passage that matched when the head is not it.
type FinderRow struct {
	Rect     city.Rect
	Head     Text
	Where    Text
	Passage  Text
	Marks    []Mark
	Selected bool
}

// FinderPanel is the finder laid out over the city: a plate holding the
// field on its first line and the status beneath it.
type FinderPanel struct {
	Rect    city.Rect
	Fill    color.NRGBA
	Radius  float64
	Pad     float64
	Field   city.Rect
	Query   Text
	Caret   city.Rect
	Status  Text
	Rows    []FinderRow
	Preview []Text
}

// finderGrids is the panel's width when the window has room for it.
const finderGrids = 80

// LayoutFinder centres the finder across a width×height window and
// hangs it in the top third, the way a command palette sits. A query
// wider than the field scrolls so the caret stays in view.
func LayoutFinder(th Theme, width, height float64, in FinderInput, measure Measure) FinderPanel {
	grid := th.Grid()
	pad := 2 * grid
	w := min(finderGrids*grid, width-2*grid)
	_, titleH := measure("", Title)
	_, smallH := measure("", Small)
	h := pad + titleH + grid + smallH + pad
	p := FinderPanel{
		Rect:   city.RectAt((width-w)/2, height/6, w, h),
		Fill:   th.Palette.Panel,
		Radius: th.Radius(),
		Pad:    pad,
	}
	p.Field = city.RectAt(p.Rect.Min.X+pad, p.Rect.Min.Y+pad, w-2*pad, titleH)
	caretW := 2 * th.Hairline()
	runes := []rune(in.Query)
	caret := max(0, min(in.Caret, len(runes)))
	start := 0
	for start < caret {
		if before, _ := measure(string(runes[start:caret]), Title); before <= p.Field.Width()-caretW {
			break
		}
		start++
	}
	before, _ := measure(string(runes[start:caret]), Title)
	shown := clipTo(string(runes[start:]), p.Field.Width(), Title, measure)
	p.Query = Text{Text: shown, At: p.Field.Min, Size: Title}
	p.Caret = city.RectAt(p.Field.Min.X+before, p.Field.Min.Y, caretW, titleH)
	p.Status = Text{Text: in.Status, At: city.Point{X: p.Field.Min.X, Y: p.Field.Max.Y + grid}, Size: Small}
	if len(in.Results) == 0 {
		return p
	}
	p.layoutResults(th, height, in, measure)
	return p
}

// finderPreviewRows is how many lines of recap the preview shows.
const finderPreviewRows = 3

// layoutResults lays the rows under the status line and the selected
// row's preview under them, as many rows as fit above the window's
// bottom, scrolled so the selected one is among them.
func (p *FinderPanel) layoutResults(th Theme, height float64, in FinderInput, measure Measure) {
	grid := th.Grid()
	width := p.Field.Width()
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	selected := max(0, min(in.Selected, len(in.Results)-1))
	var preview []string
	if r := in.Results[selected]; r.Recap != "" || r.LastPrompt != "" {
		preview = wrapPara(r.Recap, width, finderPreviewRows, measure)
		if r.LastPrompt != "" {
			preview = append(preview, Fit("left at: "+strings.Join(strings.Fields(r.LastPrompt), " "), width, Small, measure))
		}
	}
	previewH := 0.0
	if len(preview) > 0 {
		previewH = grid + smallH*float64(len(preview))
	}
	rowH := func(r FinderResult) float64 {
		h := grid + bodyH + smallH
		if !r.InPrompt {
			h += smallH
		}
		return h
	}
	top := p.Status.At.Y + smallH + grid
	room := height - 2*grid - p.Pad - previewH - top
	first, used := selected, rowH(in.Results[selected])
	for first > 0 && used+rowH(in.Results[first-1]) <= room {
		first--
		used += rowH(in.Results[first])
	}
	last := selected
	for last+1 < len(in.Results) && used+rowH(in.Results[last+1]) <= room {
		last++
		used += rowH(in.Results[last])
	}
	y := top
	for i := first; i <= last; i++ {
		r := in.Results[i]
		row := FinderRow{Rect: city.RectAt(p.Rect.Min.X, y, p.Rect.Width(), rowH(r)), Selected: i == selected}
		head := r.Passage
		if !r.InPrompt {
			head = r.FirstPrompt
		}
		if head == "" {
			head = "no prompt kept"
		}
		x := p.Field.Min.X
		row.Head = Text{Text: around(oneLine(head), in.Words, width, Body, measure), At: city.Point{X: x, Y: y + grid/2}, Size: Body}
		where := []string{r.Dir}
		if r.Branch != "" {
			where = append(where, r.Branch)
		}
		if r.Group {
			where = append(where, r.Age)
		} else {
			where = append(where, r.Age+" ago")
		}
		if r.Gone {
			where = append(where, "conversation deleted")
		}
		if r.Packed {
			where = append(where, "packed up, enter unpacks")
		}
		row.Where = Text{Text: Fit(strings.Join(where, "  "), width, Small, measure), At: city.Point{X: x, Y: row.Head.At.Y + bodyH}, Size: Small}
		row.Marks = marks(row.Head, in.Words, measure)
		if !r.InPrompt {
			row.Passage = Text{Text: around(oneLine(r.Passage), in.Words, width, Small, measure), At: city.Point{X: x, Y: row.Where.At.Y + smallH}, Size: Small}
			row.Marks = append(row.Marks, marks(row.Passage, in.Words, measure)...)
		}
		p.Rows = append(p.Rows, row)
		y += row.Rect.Height()
	}
	if len(preview) > 0 {
		y += grid
		for _, line := range preview {
			p.Preview = append(p.Preview, Text{Text: line, At: city.Point{X: p.Field.Min.X, Y: y}, Size: Small})
			y += smallH
		}
	}
	p.Rect.Max.Y = y + p.Pad
}

// around fits a line to width so that its first match is in view:
// clipped at the end when the match is near the start, and otherwise
// begun with an ellipsis a little before the match.
func around(text string, words []string, width float64, size Size, measure Measure) string {
	lower := strings.ToLower(text)
	at, end := -1, 0
	for _, w := range words {
		if i := strings.Index(lower, strings.ToLower(w)); i >= 0 && (at < 0 || i < at) && len(lower) == len(text) {
			at, end = i, i+len(w)
		}
	}
	if at < 0 {
		return Fit(text, width, size, measure)
	}
	if upto, _ := measure(text[:end], size); upto <= width {
		return Fit(text, width, size, measure)
	}
	start := at
	for start > 0 {
		_, n := utf8.DecodeLastRuneInString(text[:start])
		if lead, _ := measure(text[start-n:at], size); lead > width/3 {
			break
		}
		start -= n
	}
	if i := strings.IndexByte(text[start:at], ' '); i >= 0 && start > 0 {
		start += i + 1
	}
	return Fit("…"+text[start:], width, size, measure)
}

// oneLine folds a passage's whitespace, newlines included, to spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// marks finds each word wherever it appears in a drawn line, ignoring
// case, and returns the spans to highlight.
func marks(line Text, words []string, measure Measure) []Mark {
	lower := strings.ToLower(line.Text)
	if len(lower) != len(line.Text) {
		return nil
	}
	var out []Mark
	for _, w := range words {
		w = strings.ToLower(w)
		if w == "" {
			continue
		}
		for from := 0; ; {
			i := strings.Index(lower[from:], w)
			if i < 0 {
				break
			}
			i += from
			start, _ := measure(line.Text[:i], line.Size)
			end, _ := measure(line.Text[:i+len(w)], line.Size)
			out = append(out, Mark{Line: line, From: line.At.X + start, To: line.At.X + end})
			from = i + len(w)
		}
	}
	return out
}
