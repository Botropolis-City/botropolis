package ui

import (
	"slices"

	"botropolis.city/botropolis/pkg/city"
)

// Dock is where the selection's card goes while the sidebar is open:
// the card, where the list above it ends, and whether it stands beside
// the panel or is docked in its compact form (item 140).
type Dock struct {
	Card            Card
	Foot            float64
	Beside, Compact bool
}

// sidebarFloorRows is how many rows the list keeps however short the
// window.
const sidebarFloorRows = 4

// SidebarFloor is the least height the sidebar keeps for its list: four
// rows between both scroll marks, and the panel's padding.
func SidebarFloor(th Theme, measure Measure) float64 {
	_, bodyH := measure("", Body)
	rowH := bodyH + th.Grid()
	return (sidebarFloorRows+2)*rowH + 2*th.Grid()
}

// DockCard places the selection's card for a sidebar spanning panel,
// trying in turn: the whole card docked at the panel's foot, when that
// leaves the list its floor; the whole card beside the panel, in room
// and clear of every cover; with its details closed, the compact card
// docked at the foot, kind, title and verbs; and last the card beside
// the panel, over whatever is there.
func DockCard(th Theme, card city.Card, panel, room city.Rect, covers []city.Rect, measure Measure) Dock {
	grid := th.Grid()
	floor := SidebarFloor(th, measure)
	docked := func(c Card) (Dock, bool) {
		c = c.MoveTo(city.Point{X: grid, Y: panel.Max.Y - grid - c.Rect.Height()})
		foot := c.Rect.Min.Y - grid
		return Dock{Card: c, Foot: foot}, foot-panel.Min.Y >= floor
	}
	whole := LayoutCard(th, card, panel, measure)
	if d, ok := docked(whole); ok {
		return d
	}
	beside := func(c Card, x float64) Dock {
		y := max(panel.Max.Y-grid-c.Rect.Height(), panel.Min.Y)
		return Dock{Card: c.MoveTo(city.Point{X: x, Y: y}), Foot: panel.Max.Y, Beside: true}
	}
	xs := []float64{panel.Max.X + grid}
	for _, c := range covers {
		xs = append(xs, c.Max.X+grid)
	}
	slices.Sort(xs)
	for _, x := range xs {
		d := beside(whole, x)
		if d.Card.Rect.Max.X <= room.Max.X && !overlapsAny(d.Card.Rect, covers) {
			return d
		}
	}
	if !card.Open {
		compact := city.Card{Kind: card.Kind, Title: card.Title, Move: card.Move, Group: card.Group, Actions: card.Actions, Hint: card.Hint}
		if d, ok := docked(LayoutCard(th, compact, panel, measure)); ok {
			d.Compact = true
			return d
		}
	}
	return beside(whole, panel.Max.X+grid)
}

func overlapsAny(r city.Rect, covers []city.Rect) bool {
	for _, c := range covers {
		if c.Area() > 0 && r.Overlaps(c) {
			return true
		}
	}
	return false
}
