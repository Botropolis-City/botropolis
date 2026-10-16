package ui

import (
	"math"
	"sort"

	"botropolis.city/botropolis/pkg/city"
)

// Seen is one thing of a kind on screen, where it was drawn.
type Seen struct {
	Kind city.Kind
	Rect city.Rect
}

// Caption is one kind's caption, on a plate beside one of its kind.
type Caption struct {
	Kind  city.Kind
	Text  string
	Plate Plate
}

// LayoutExplain is explain mode (item 88): while the key is held, each
// kind on screen gets one caption, on the one of its kind nearest the
// middle of the window, moved down past any caption it would cover and
// kept inside the window.
func LayoutExplain(th Theme, seen []Seen, window city.Rect, held bool, measure Measure) []Caption {
	if !held {
		return nil
	}
	centre := window.Center()
	best := map[city.Kind]Seen{}
	for _, s := range seen {
		if s.Kind == city.KindNone {
			continue
		}
		if b, ok := best[s.Kind]; !ok || dist(s.Rect.Center(), centre) < dist(b.Rect.Center(), centre) {
			best[s.Kind] = s
		}
	}
	kinds := make([]city.Kind, 0, len(best))
	for k := range best {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool {
		return dist(best[kinds[i]].Rect.Center(), centre) < dist(best[kinds[j]].Rect.Center(), centre)
	})
	gap := th.Grid() / 2
	var caps []Caption
	for _, k := range kinds {
		s := best[k]
		text := k.Caption()
		w, h := measure(text, Small)
		pw, ph := w+th.Grid(), h+th.Grid()/2
		x := math.Min(math.Max(s.Rect.Center().X-pw/2, window.Min.X), window.Max.X-pw)
		y := math.Min(math.Max(s.Rect.Max.Y+gap, window.Min.Y), window.Max.Y-ph)
		placed := false
		for range len(kinds) + 1 {
			r := city.RectAt(x, y, pw, ph)
			clash := false
			for _, c := range caps {
				if c.Plate.Rect.Overlaps(r) {
					clash, y = true, c.Plate.Rect.Max.Y+gap
					break
				}
			}
			if !clash && r.Max.Y <= window.Max.Y {
				placed = true
				break
			}
			if y+ph > window.Max.Y {
				break
			}
		}
		if !placed {
			continue
		}
		caps = append(caps, Caption{Kind: k, Text: text, Plate: LayoutPlate(th, text, city.Point{X: x, Y: y}, Small, measure)})
	}
	return caps
}

func dist(a, b city.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }
