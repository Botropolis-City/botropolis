package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351: the line under the menu says what the hovered or focused
// row does and ends with the keys that run it, as caps after a dot.
func TestMenuDescriptionCap(t *testing.T) {
	th := ui.NewTheme(1)
	bounds := city.RectAt(0, 0, 1000, 800)
	columns := []ui.MenuColumn{
		{Title: "Session", Key: "s", Rows: []ui.MenuRow{
			{Label: "attach", Key: "a", Path: []string{"space", "s", "a"}, Describe: "Opens this session's terminal."},
			{Label: "pack up", Key: "p", Path: []string{"space", "s", "p"}, Describe: "Takes this session off the map and keeps it in the depot, to be brought back later."},
		}},
		{Title: "Project", Key: "p", Rows: []ui.MenuRow{
			{Label: "reveal folder", Key: "f", Describe: "Opens this project's folder."},
		}},
	}
	away := city.Point{X: -1, Y: -1}
	labels := func(caps []ui.KeyCap) []string {
		var out []string
		for _, c := range caps {
			out = append(out, c.Label.Text)
		}
		return out
	}

	t.Run("when a row is focused and nothing is hovered", func(t *testing.T) {
		m := ui.LayoutMenu(th, columns, ui.MenuFocus{Column: 0, Row: 1}, away, city.Point{X: 100, Y: 100}, bounds, measure7)
		require.NotEmpty(t, m.Description)
		last := m.Description[len(m.Description)-1]

		t.Run("it should end the description with the row's path as caps", func(t *testing.T) {
			assert.Equal(t, []string{"space", "s", "p"}, labels(m.DescriptionCaps))
		})

		t.Run("it should set a dot before the caps on their row", func(t *testing.T) {
			assert.Equal(t, []any{"·", m.DescriptionCaps[0].Label.At.Y}, []any{m.DescriptionDot.Text, m.DescriptionDot.At.Y})
		})

		t.Run("it should set the caps no higher than the description's last line", func(t *testing.T) {
			assert.GreaterOrEqual(t, m.DescriptionCaps[0].Label.At.Y, last.At.Y)
		})

		t.Run("it should keep the caps inside the menu", func(t *testing.T) {
			for _, c := range m.DescriptionCaps {
				assert.Equal(t, m.Rect, m.Rect.Union(c.Rect), "cap %q at %v", c.Label.Text, c.Rect)
			}
		})

		t.Run("and the menu is moved", func(t *testing.T) {
			moved := m.MoveTo(m.Rect.Min.Add(city.Point{X: 10, Y: 20}))

			t.Run("it should carry the caps", func(t *testing.T) {
				assert.Equal(t, m.DescriptionCaps[0].Rect.Min.Add(city.Point{X: 10, Y: 20}), moved.DescriptionCaps[0].Rect.Min)
			})
		})
	})

	t.Run("when a row is hovered", func(t *testing.T) {
		probe := ui.LayoutMenu(th, columns, ui.NoMenuFocus, away, city.Point{X: 100, Y: 100}, bounds, measure7)
		m := ui.LayoutMenu(th, columns, ui.MenuFocus{Column: 0, Row: 1}, probe.Rows[0][0].Rect.Center(), city.Point{X: 100, Y: 100}, bounds, measure7)

		t.Run("it should end the description with the hovered row's caps", func(t *testing.T) {
			assert.Equal(t, []string{"space", "s", "a"}, labels(m.DescriptionCaps))
		})

		t.Run("it should be the same size as with the longest row focused", func(t *testing.T) {
			focused := ui.LayoutMenu(th, columns, ui.MenuFocus{Column: 0, Row: 1}, away, city.Point{X: 100, Y: 100}, bounds, measure7)
			assert.Equal(t, focused.Rect.Size(), m.Rect.Size())
		})
	})

	t.Run("when the described row has no path", func(t *testing.T) {
		m := ui.LayoutMenu(th, columns, ui.MenuFocus{Column: 1, Row: 0}, away, city.Point{X: 100, Y: 100}, bounds, measure7)

		t.Run("it should set no dot", func(t *testing.T) {
			assert.Empty(t, m.DescriptionDot.Text)
		})

		t.Run("it should set no caps", func(t *testing.T) {
			assert.Empty(t, m.DescriptionCaps)
		})
	})
}
