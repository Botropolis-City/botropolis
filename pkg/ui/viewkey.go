package ui

import (
	"strconv"

	"botropolis.city/botropolis/pkg/city"
)

// The key to the lenses: what there is to look at, and how to get there.
//
// The nine views were reachable only by keys you had to know existed,
// which is the same failure the movers had one level up — a thing on
// screen that cannot be asked what it is. A legend tells you what the
// view you are already in means; it cannot tell you what else there is.
//
// So this lists the six lenses (item 88), which fold those views, with
// their numbers and what each shows, marks
// the one you are in, and each row is clickable, because a reader who
// does not know the number should not have to learn one to move.

// ViewKeyRow is one lens in the key.
type ViewKeyRow struct {
	Lens     city.Lens
	Key      string
	Name     string
	Question string
	// Row is the whole band, so a click anywhere along it counts rather
	// than only on the words.
	Row        city.Rect
	Chip       city.Rect
	KeyAt      city.Point
	NameAt     city.Point
	QuestionAt city.Point
	Current    bool
}

// ViewKey is the panel placed for one window.
type ViewKey struct {
	Rect  city.Rect
	Title Text
	Size  Size
	Rows  []ViewKeyRow
	// Note names the two things that share the river, because they look
	// alike enough to need telling apart and the key is where the map
	// explains its own vocabulary. Item 49.
	Note Text
}

// RiverNote is what the key says about the river. Two meanings on one
// waterway are only allowed if each says what it is, and this is the
// half of that which does not require hovering.
//
// It names the buoys because after bug 55 they are the scale's ends
// rather than the river's: the gauge runs between the first and the
// last, not from bank to bank nor from one end of the water to the
// other. A mark that means something other than where it sits has to
// say so somewhere, and this is where the map explains its vocabulary.
const RiverNote = "on the river: liners gauge your usage between the buoys, tugs are sessions coming and going, the launch is " + city.RefreshKey + " fetching fresh figures"

// LayoutViewKey centres the key in a width×height window and marks the
// lens that is up.
func LayoutViewKey(th Theme, width, height float64, current city.Lens, measure Measure) ViewKey {
	grid := th.Grid()
	pad := 3 * grid
	_, lineH := measure("", Small)
	title := "Lenses"
	_, titleH := measure(title, Title)
	chipH := lineH + grid

	var chipW, nameW, questionW float64
	for _, l := range city.Lenses {
		if w, _ := measure(l.Name(), Small); w > nameW {
			nameW = w
		}
		if w, _ := measure(l.About(), Small); w > questionW {
			questionW = w
		}
	}
	if w, _ := measure("9", Small); w+grid > chipW {
		chipW = w + grid
	}
	rowStep := chipH + grid/2
	noteW, noteH := measure(RiverNote, Small)
	w := pad + chipW + grid + nameW + 2*grid + questionW + pad
	if want := pad + noteW + pad; want > w {
		w = want
	}
	h := pad + titleH + grid + rowStep*float64(len(city.Lenses)) - grid/2 + grid + noteH + pad
	k := ViewKey{Rect: city.RectAt((width-w)/2, (height-h)/2, w, h), Size: Small}
	k.Title = Text{Text: title, At: k.Rect.Min.Add(city.Point{X: pad, Y: pad}), Size: Title}

	y := k.Rect.Min.Y + pad + titleH + grid
	for i, l := range city.Lenses {
		chip := city.RectAt(k.Rect.Min.X+pad, y, chipW, chipH)
		k.Rows = append(k.Rows, ViewKeyRow{
			Lens:       l,
			Key:        strconv.Itoa(i + 1),
			Name:       l.Name(),
			Question:   l.About(),
			Row:        city.RectAt(k.Rect.Min.X+grid, y, k.Rect.Width()-2*grid, chipH),
			Chip:       chip,
			KeyAt:      city.Point{X: chip.Min.X + grid/2, Y: y + grid/2},
			NameAt:     city.Point{X: chip.Max.X + grid, Y: y + grid/2},
			QuestionAt: city.Point{X: chip.Max.X + grid + nameW + 2*grid, Y: y + grid/2},
			Current:    l == current,
		})
		y += rowStep
	}
	k.Note = Text{Text: RiverNote, At: city.Point{X: k.Rect.Min.X + pad, Y: y - grid/2 + grid}, Size: Small}
	return k
}

// Hit is the lens a click lands on, if it lands on a row at all. A click
// between rows chooses nothing rather than the nearest, because a menu
// that acts on a near miss is worse than one that ignores it.
func (k ViewKey) Hit(at city.Point) (city.Lens, bool) {
	for _, r := range k.Rows {
		if r.Row.Contains(at) {
			return r.Lens, true
		}
	}
	return city.LensAttention, false
}
