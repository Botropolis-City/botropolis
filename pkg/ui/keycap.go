package ui

import "botropolis.city/botropolis/pkg/city"

// KeyCap is a key as drawn on screen: its name in Small type on a chip,
// the one look for a key wherever it shows (item 352). It knows nothing
// about bindings; its caller names the key.
type KeyCap struct {
	Rect  city.Rect
	Label Text
}

// LayoutKeyCap lays a cap for key with its top-left corner at.
func LayoutKeyCap(th Theme, key string, at city.Point, measure Measure) KeyCap {
	grid := th.Grid()
	w, h := measure(key, Small)
	return KeyCap{
		Rect:  city.RectAt(at.X, at.Y, w+grid, h+grid),
		Label: Text{Text: key, At: city.Point{X: at.X + grid/2, Y: at.Y + grid/2}, Size: Small},
	}
}

// Moved is the cap with its top-left corner at min.
func (c KeyCap) Moved(min city.Point) KeyCap {
	d := min.Sub(c.Rect.Min)
	c.Rect = city.Rect{Min: c.Rect.Min.Add(d), Max: c.Rect.Max.Add(d)}
	c.Label = c.Label.moved(d)
	return c
}

// LayoutKeyCaps lays one cap per press of a sequence, left to right
// from at, half a grid apart (item 351).
func LayoutKeyCaps(th Theme, keys []string, at city.Point, measure Measure) []KeyCap {
	var caps []KeyCap
	x := at.X
	for _, k := range keys {
		c := LayoutKeyCap(th, k, city.Point{X: x, Y: at.Y}, measure)
		caps = append(caps, c)
		x = c.Rect.Max.X + th.Grid()/2
	}
	return caps
}

// KeyCapsWidth is how wide LayoutKeyCaps lays keys.
func KeyCapsWidth(th Theme, keys []string, measure Measure) float64 {
	caps := LayoutKeyCaps(th, keys, city.Point{}, measure)
	if len(caps) == 0 {
		return 0
	}
	return caps[len(caps)-1].Rect.Max.X
}

// keyDot is what parts a line from the key that ends it.
const keyDot = "·"

// keyTail is how wide a line's ending key is: a grid, the dot, a grid
// and the caps; none without keys.
func keyTail(th Theme, keys []string, measure Measure) float64 {
	if len(keys) == 0 {
		return 0
	}
	dotW, _ := measure(keyDot, Small)
	return th.Grid() + dotW + th.Grid() + KeyCapsWidth(th, keys, measure)
}

// keyedLines wraps text into rows no wider than width and says where
// its key goes: on the last row when it fits after the text, else at
// the start of a row of its own (item 351).
func keyedLines(th Theme, text string, keys []string, width float64, rows int, measure Measure) (lines []string, keyRow int, keyX float64) {
	lines = wrapAt(text, width, rows, Small, measure)
	if len(keys) == 0 {
		return lines, -1, 0
	}
	tail := keyTail(th, keys, measure)
	if len(lines) > 0 {
		lastW, _ := measure(lines[len(lines)-1], Small)
		if lastW+tail <= width {
			return lines, len(lines) - 1, lastW
		}
	}
	return lines, len(lines), ownRow
}

// ownRow is the x keyedLines gives a key on a row of its own, which
// layKey sets with no dot, since nothing on the row comes before it.
const ownRow = -1

// keyedRows is how many rows keyedLines takes for text and its key.
func keyedRows(th Theme, text string, keys []string, width float64, measure Measure) int {
	lines, row, _ := keyedLines(th, text, keys, width, int(^uint(0)>>1), measure)
	return max(len(lines), row+1)
}

// layKey sets a line's ending key: the dot a grid after x, and the caps
// a grid after the dot, the dot's text on the row whose text starts at
// at; on a row of its own, the caps alone at the row's start.
func layKey(th Theme, keys []string, at city.Point, x float64, measure Measure) (Text, []KeyCap) {
	if len(keys) == 0 {
		return Text{}, nil
	}
	grid := th.Grid()
	if x == ownRow {
		return Text{}, LayoutKeyCaps(th, keys, city.Point{X: at.X, Y: at.Y - grid/2}, measure)
	}
	dot := Text{Text: keyDot, At: city.Point{X: at.X + x + grid, Y: at.Y}, Size: Small}
	dotW, _ := measure(keyDot, Small)
	return dot, LayoutKeyCaps(th, keys, city.Point{X: dot.At.X + dotW + grid, Y: at.Y - grid/2}, measure)
}
