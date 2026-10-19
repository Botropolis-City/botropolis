package ui

import (
	"strings"

	"botropolis.city/botropolis/pkg/city"
)

// The reference behind ? (item 88): an entry for every kind of thing on
// the map, organised by object rather than by key, whose terms open
// their own entries; the key list is one of its entries.

const (
	// IndexEntry is the entry ? opens on: every term there is.
	IndexEntry = "index"
	// KeysEntry is every key the city answers to.
	KeysEntry = "keys"
)

// RefEntry is one entry: its title, what it says, and the terms it
// links to by ID.
type RefEntry struct {
	ID    string
	Title string
	Is    string
	Lines []string
	Terms []string
	// Credit is the small print at the entry's foot: the index's thanks
	// to the people whose work the city is drawn with. Item 122.
	Credit string
}

// Reference is the entries and where the reader has been.
type Reference struct {
	entries map[string]RefEntry
	history []string
}

// NewReference builds the entries, the key list among them, and opens
// on the index, which carries the credit.
func NewReference(keys []Key, credit string) *Reference {
	r := &Reference{entries: map[string]RefEntry{}}
	index := RefEntry{ID: IndexEntry, Title: "Reference", Is: "everything on the map, and every key", Credit: credit}
	for _, k := range city.Kinds {
		e := RefEntry{ID: k.Name(), Title: k.Name(), Is: k.Is(), Lines: []string{k.About()}}
		for _, rel := range k.Related() {
			e.Terms = append(e.Terms, rel.Name())
		}
		r.entries[e.ID] = e
		index.Terms = append(index.Terms, e.ID)
	}
	ke := RefEntry{ID: KeysEntry, Title: "keys", Is: "every key the city answers to"}
	for _, k := range keys {
		ke.Lines = append(ke.Lines, k.Key+" — "+k.Action)
	}
	r.entries[KeysEntry] = ke
	index.Terms = append(index.Terms, KeysEntry)
	r.entries[IndexEntry] = index
	r.history = []string{IndexEntry}
	return r
}

// Entry is the entry with an ID.
func (r *Reference) Entry(id string) (RefEntry, bool) {
	e, ok := r.entries[id]
	return e, ok
}

// Current is the entry open now.
func (r *Reference) Current() RefEntry { return r.entries[r.history[len(r.history)-1]] }

// Open opens an entry, remembering the one it came from.
func (r *Reference) Open(id string) {
	if _, ok := r.entries[id]; ok && id != r.Current().ID {
		r.history = append(r.history, id)
	}
}

// Back returns to the entry before, and reports whether there was one.
func (r *Reference) Back() bool {
	if len(r.history) < 2 {
		return false
	}
	r.history = r.history[:len(r.history)-1]
	return true
}

// ReferencePanel is the open entry laid out.
type ReferencePanel struct {
	Rect     city.Rect
	Title    Text
	Subtitle Text
	Lines    []Text
	Terms    []Button
	termIDs  []string
	Hint     Text
	Credit   []Text
}

// LayoutReference centres the open entry in the window.
func LayoutReference(th Theme, width, height float64, r *Reference, measure Measure) ReferencePanel {
	e := r.Current()
	grid := th.Grid()
	pad := 3 * grid
	w := min(ParaWidth(th)+2*pad, width-4*grid)
	inner := w - 2*pad
	_, titleH := measure("", Title)
	_, smallH := measure("", Small)
	_, bodyH := measure("", Body)
	var lines []string
	for _, l := range e.Lines {
		lines = append(lines, wrapPara(l, inner, 12, measure)...)
	}
	p := ReferencePanel{}
	y := pad
	p.Title = Text{Text: e.Title, At: city.Point{X: pad, Y: y}, Size: Title}
	y += titleH
	p.Subtitle = Text{Text: e.Is, At: city.Point{X: pad, Y: y}, Size: Small}
	y += smallH + grid
	for _, l := range lines {
		p.Lines = append(p.Lines, Text{Text: l, At: city.Point{X: pad, Y: y}, Size: Body})
		y += bodyH
	}
	if len(e.Terms) > 0 {
		y += grid
		row := LayoutButtonRows(th, e.Terms, city.Point{X: pad, Y: y}, inner, measure)
		p.Terms, p.termIDs = row, append([]string{}, e.Terms...)
		for _, b := range row {
			y = max(y, b.Rect.Max.Y)
		}
		y += grid
	}
	hint := "click a term to open it"
	if len(r.history) > 1 {
		hint += "; backspace goes back"
	}
	p.Hint = Text{Text: strings.TrimSpace(hint), At: city.Point{X: pad, Y: y}, Size: Small}
	y += smallH
	if credit := wrapAt(e.Credit, inner, len(e.Credit), Small, measure); len(credit) > 0 {
		y += grid
		for _, l := range credit {
			p.Credit = append(p.Credit, Text{Text: l, At: city.Point{X: pad, Y: y}, Size: Small})
			y += smallH
		}
	}
	y += pad
	h := min(y, height-4*grid)
	at := city.Point{X: (width - w) / 2, Y: (height - h) / 2}
	return p.moved(at, w, h)
}

func (p ReferencePanel) moved(at city.Point, w, h float64) ReferencePanel {
	p.Rect = city.RectAt(at.X, at.Y, w, h)
	p.Title, p.Subtitle, p.Hint = p.Title.moved(at), p.Subtitle.moved(at), p.Hint.moved(at)
	for i := range p.Lines {
		p.Lines[i] = p.Lines[i].moved(at)
	}
	for i := range p.Credit {
		p.Credit[i] = p.Credit[i].moved(at)
	}
	for i, b := range p.Terms {
		p.Terms[i] = Button{Rect: city.Rect{Min: b.Rect.Min.Add(at), Max: b.Rect.Max.Add(at)}, Label: b.Label.moved(at)}
	}
	return p
}

// TermRect is where a term's button is, by the entry's ID.
func (p ReferencePanel) TermRect(id string) (city.Rect, bool) {
	for i, t := range p.termIDs {
		if t == id {
			return p.Terms[i].Rect, true
		}
	}
	return city.Rect{}, false
}

// Hit is the term a click lands on.
func (p ReferencePanel) Hit(at city.Point) (string, bool) {
	for i, b := range p.Terms {
		if b.Rect.Contains(at) {
			return p.termIDs[i], true
		}
	}
	return "", false
}
