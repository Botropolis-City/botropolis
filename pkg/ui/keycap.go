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
