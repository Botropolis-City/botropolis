package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestSidebarFloor(t *testing.T) {
	t.Run("when a long list is scrolled into its middle", func(t *testing.T) {
		th := ui.NewTheme(1)
		sb := ui.LayoutSidebar(th, 0, ui.SidebarFloor(th, measure7), manyProjects(10), nil, "", 5, false, measure7)

		t.Run("it should leave room for four rows between both marks", func(t *testing.T) {
			assert.Len(t, sb.Rows, 4)
		})
	})
}

func faceCard(open bool) city.Card {
	return city.Card{
		Kind:     "session",
		Title:    "Fix the CI queue",
		Group:    "cinders",
		Sentence: strings.Repeat("It is waiting on you to approve a command. ", 4),
		Numbers:  []city.Number{{Label: "tokens", Value: "12k"}, {Label: "cost", Value: "$1"}},
		Series:   []float64{1, 3, 2, 5, 4},
		Actions:  []string{"attach", "stop", "rest", "done"},
		Details:  []string{"model opus", "branch main", "cwd /p/cinders", "started 09:00"},
		Open:     open,
	}
}

func TestDockCard(t *testing.T) {
	th := ui.NewTheme(1)
	grid := th.Grid()
	floor := ui.SidebarFloor(th, measure7)
	const top, width = 32.0, 1024.0
	panelOf := func(h float64) city.Rect { return city.RectAt(0, top, ui.SidebarWidth(th), h) }
	whole := ui.LayoutCard(th, faceCard(false), panelOf(1000), measure7).Rect.Height()
	f := faceCard(false)
	compact := ui.LayoutCard(th, city.Card{Kind: f.Kind, Title: f.Title, Group: f.Group, Actions: f.Actions}, panelOf(1000), measure7).Rect.Height()
	lens := city.RectAt(ui.SidebarWidth(th)+2*grid, top+2*grid, 392, 140)
	stream := city.RectAt(width-256, top, 256, 192)

	cases := []struct {
		when, should string
		height       float64
		open         bool
		covers       []city.Rect
		beside       bool
		compact      bool
		x            float64
		taller       bool
	}{
		{when: "when the whole card leaves the list its floor", should: "it should dock the whole card at the panel's foot",
			height: whole + floor + 4*grid, x: grid},
		{when: "when the whole card would leave the list under its floor and nothing covers beside the panel", should: "it should stand the whole card beside the panel",
			height: whole + 4*grid, beside: true, x: ui.SidebarWidth(th) + grid},
		{when: "when the lens panel covers beside the panel and the room past it is clear", should: "it should stand the whole card past the lens panel",
			height: whole + 4*grid, covers: []city.Rect{lens}, beside: true, x: lens.Max.X + grid},
		{when: "when no place beside is clear and the details are closed", should: "it should dock the compact card at the panel's foot",
			height: whole + 4*grid, covers: []city.Rect{lens, stream}, compact: true, x: grid},
		{when: "when no place beside is clear and the details are open", should: "it should stand the whole card beside the panel, over the lens panel",
			height: whole + 4*grid, open: true, covers: []city.Rect{lens, stream}, beside: true, x: ui.SidebarWidth(th) + grid},
		{when: "when even the compact card would leave the list under its floor", should: "it should stand the card beside the panel, over the lens panel",
			height: floor + compact, covers: []city.Rect{lens, stream}, beside: true, x: ui.SidebarWidth(th) + grid, taller: true},
	}
	for _, c := range cases {
		t.Run(c.when, func(t *testing.T) {
			panel := panelOf(c.height)
			room := city.Rect{Min: city.Point{X: panel.Max.X, Y: panel.Min.Y}, Max: city.Point{X: width, Y: panel.Max.Y}}
			dock := ui.DockCard(th, faceCard(c.open), panel, room, c.covers, measure7)
			list := city.Rect{Min: panel.Min, Max: city.Point{X: panel.Max.X, Y: dock.Foot}}

			t.Run(c.should, func(t *testing.T) {
				assert.Equal(t, []any{c.beside, c.compact, c.x}, []any{dock.Beside, dock.Compact, dock.Card.Rect.Min.X})
			})

			if !c.taller {
				t.Run("it should sit the card's bottom a grid above the panel's", func(t *testing.T) {
					assert.InDelta(t, panel.Max.Y-grid, dock.Card.Rect.Max.Y, 0.01)
				})
			}

			t.Run("it should keep the card's top under the panel's", func(t *testing.T) {
				assert.GreaterOrEqual(t, dock.Card.Rect.Min.Y, panel.Min.Y)
			})

			t.Run("it should keep the card clear of the list", func(t *testing.T) {
				assert.False(t, dock.Card.Rect.Overlaps(list))
			})

			if !c.beside {
				t.Run("it should leave the list its floor", func(t *testing.T) {
					assert.GreaterOrEqual(t, list.Height(), floor)
				})
			} else {
				t.Run("it should give the list the panel's whole height", func(t *testing.T) {
					assert.Equal(t, panel.Max.Y, dock.Foot)
				})
			}
		})
	}

	t.Run("when the compact card is docked", func(t *testing.T) {
		panel := panelOf(whole + 4*grid)
		room := city.Rect{Min: city.Point{X: panel.Max.X, Y: panel.Min.Y}, Max: city.Point{X: width, Y: panel.Max.Y}}
		dock := ui.DockCard(th, faceCard(false), panel, room, []city.Rect{lens, stream}, measure7)
		require.True(t, dock.Compact)

		t.Run("it should keep the card's verbs", func(t *testing.T) {
			assert.Len(t, dock.Card.Buttons, 4)
		})

		t.Run("it should drop the card's sentence", func(t *testing.T) {
			assert.Empty(t, dock.Card.Sentence)
		})
	})
}
