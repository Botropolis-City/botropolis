package ui

import "botropolis.city/botropolis/pkg/city"

// PlacedKey is a key's cap and the word beside it on a legend.
type PlacedKey struct {
	Key
	Cap     KeyCap
	LabelAt city.Point
}

// KeyLegend is a mode's keys where they show nowhere else (item 351): a
// cap and the word for what it does, pair after pair on one line.
type KeyLegend struct {
	Rect city.Rect
	Keys []PlacedKey
}

// LayoutKeyLegend lays keys on one line from at, no wider than width,
// dropping the pairs that do not fit.
func LayoutKeyLegend(th Theme, keys []Key, at city.Point, width float64, measure Measure) KeyLegend {
	grid := th.Grid()
	l := KeyLegend{Rect: city.Rect{Min: at, Max: at}}
	x := at.X
	for _, k := range keys {
		lw, _ := measure(k.Action, Small)
		cap := LayoutKeyCap(th, k.Key, city.Point{X: x, Y: at.Y}, measure)
		labelX := cap.Rect.Max.X + grid
		if labelX+lw > at.X+width {
			break
		}
		l.Keys = append(l.Keys, PlacedKey{Key: k, Cap: cap, LabelAt: city.Point{X: labelX, Y: cap.Label.At.Y}})
		l.Rect = l.Rect.Union(cap.Rect)
		l.Rect.Max.X = labelX + lw
		x = labelX + lw + 3*grid
	}
	return l
}

// Moved is the legend with its top-left corner at min.
func (l KeyLegend) Moved(min city.Point) KeyLegend {
	d := min.Sub(l.Rect.Min)
	moved := KeyLegend{Rect: city.Rect{Min: l.Rect.Min.Add(d), Max: l.Rect.Max.Add(d)}}
	for _, k := range l.Keys {
		moved.Keys = append(moved.Keys, PlacedKey{Key: k.Key, Cap: k.Cap.Moved(k.Cap.Rect.Min.Add(d)), LabelAt: k.LabelAt.Add(d)})
	}
	return moved
}
