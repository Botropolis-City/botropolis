package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350: the action menu is one component, three columns of rows
// with their keys, placed at a point and kept inside the window.
func TestLayoutMenu(t *testing.T) {
	th := ui.NewTheme(1)
	bounds := city.RectAt(0, 0, 1000, 800)
	columns := []ui.MenuColumn{
		{Title: "Session", Key: "s", Rows: []ui.MenuRow{
			{Label: "attach", Key: "a", Describe: "Opens this session's terminal."},
			{Label: "stop", Key: "s", Describe: "Stops this session. Its conversation is kept, and you can resume it."},
			{Label: "demolish", Key: "d d", Describe: "Deletes this session and its worktree, and takes it off the map. Its conversation stays in Claude Code's files, which is the longest line a menu row has to make room for."},
		}},
		{Title: "Project", Key: "p", Rows: []ui.MenuRow{
			{Label: "new session here", Key: "n", Describe: "Starts a new session in this project's directory."},
		}},
		{Title: "Group", Key: "g", Rows: []ui.MenuRow{
			{Label: "move to group…", Key: "m", Describe: "Moves this session into a group, or into a new one you name."},
			{Label: "focus the line", Key: "f", Describe: "Shows only this session's group, and frames its projects."},
		}},
	}
	none := ui.MenuFocus{Column: -1, Row: -1}
	away := city.Point{X: -1, Y: -1}
	laid := func(focus ui.MenuFocus, hover, at city.Point) ui.Menu {
		return ui.LayoutMenu(th, columns, focus, hover, at, bounds, measure7)
	}
	describing := func(m ui.Menu) string {
		text := ""
		for _, line := range m.Description {
			text += line.Text + " "
		}
		return text
	}

	t.Run("when a menu of three columns is laid out at a point", func(t *testing.T) {
		at := city.Point{X: 100, Y: 200}
		m := laid(none, away, at)

		t.Run("it should put its top-left corner at the point", func(t *testing.T) {
			assert.Equal(t, at, m.Rect.Min)
		})

		t.Run("it should lay the columns left to right, each heading over its rows", func(t *testing.T) {
			require.Len(t, m.Headings, 3)
			require.Len(t, m.Rows, 3)
			ordered := m.Headings[0].Title.At.X < m.Headings[1].Title.At.X && m.Headings[1].Title.At.X < m.Headings[2].Title.At.X
			over := m.Headings[0].Title.At.Y < m.Rows[0][0].Rect.Min.Y && m.Headings[2].Title.At.Y < m.Rows[2][0].Rect.Min.Y
			assert.True(t, ordered && over)
		})

		t.Run("it should right-align each row's key cap half a grid inside its column's edge", func(t *testing.T) {
			rows := m.Rows[0]
			require.Len(t, rows, 3)
			edge := rows[0].Rect.Max.X - 4
			assert.Equal(t, []float64{edge, edge, edge}, []float64{rows[0].Cap.Rect.Max.X, rows[1].Cap.Rect.Max.X, rows[2].Cap.Rect.Max.X})
		})

		t.Run("it should reserve room under the columns for the longest description, so focus never changes its height", func(t *testing.T) {
			focused := laid(ui.MenuFocus{Column: 0, Row: 2}, away, at)
			assert.Equal(t, m.Rect.Height(), focused.Rect.Height())
		})

		t.Run("it should show no description when nothing is hovered or focused", func(t *testing.T) {
			assert.Empty(t, m.Description)
		})

		t.Run("it should carry the focus it was laid with", func(t *testing.T) {
			assert.Equal(t, none, m.Focus)
		})
	})

	t.Run("when a row is focused", func(t *testing.T) {
		m := laid(ui.MenuFocus{Column: 2, Row: 1}, away, city.Point{X: 100, Y: 200})

		t.Run("it should show that row's description under the columns", func(t *testing.T) {
			require.NotEmpty(t, m.Description)
			assert.True(t, m.Description[0].At.Y > m.Rows[0][2].Rect.Max.Y && describing(m) == "Shows only this session's group, and frames its projects. ")
		})

		t.Run("it should set the description small", func(t *testing.T) {
			assert.Equal(t, ui.Small, m.Description[0].Size)
		})
	})

	t.Run("when the pointer is over a row", func(t *testing.T) {
		focused := laid(ui.MenuFocus{Column: 2, Row: 1}, away, city.Point{X: 100, Y: 200})
		m := laid(ui.MenuFocus{Column: 2, Row: 1}, focused.Rows[1][0].Rect.Center(), city.Point{X: 100, Y: 200})

		t.Run("it should show that row's description, over the focused one", func(t *testing.T) {
			assert.Equal(t, "Starts a new session in this project's directory. ", describing(m))
		})
	})

	t.Run("when the point is near the right edge", func(t *testing.T) {
		m := laid(none, away, city.Point{X: 990, Y: 200})

		t.Run("it should move left to stay inside bounds", func(t *testing.T) {
			assert.Equal(t, bounds.Max.X, m.Rect.Max.X)
		})
	})

	t.Run("when the point is near the bottom edge", func(t *testing.T) {
		m := laid(none, away, city.Point{X: 100, Y: 790})

		t.Run("it should move up to stay inside bounds", func(t *testing.T) {
			assert.Equal(t, bounds.Max.Y, m.Rect.Max.Y)
		})
	})

	t.Run("when a column has no rows", func(t *testing.T) {
		empty := []ui.MenuColumn{columns[0], {Title: "Project", Key: "p"}, columns[2]}
		m := ui.LayoutMenu(th, empty, none, away, city.Point{X: 100, Y: 200}, bounds, measure7)

		t.Run("it should still lay its heading", func(t *testing.T) {
			require.Len(t, m.Headings, 3)
			assert.Equal(t, "Project", m.Headings[1].Title.Text)
		})
	})

	t.Run("when a point is over a row", func(t *testing.T) {
		m := laid(none, away, city.Point{X: 100, Y: 200})
		hit, ok := m.Hit(m.Rows[2][1].Rect.Center())

		t.Run("it should name its column and row", func(t *testing.T) {
			require.True(t, ok)
			assert.Equal(t, ui.MenuFocus{Column: 2, Row: 1}, hit)
		})
	})

	t.Run("when a point is over a heading, or outside the menu", func(t *testing.T) {
		m := laid(none, away, city.Point{X: 100, Y: 200})
		for name, at := range map[string]city.Point{"heading": m.Headings[0].Title.At, "outside": {X: 0, Y: 0}} {
			t.Run("it should miss the "+name, func(t *testing.T) {
				_, ok := m.Hit(at)
				assert.False(t, ok)
			})
		}
	})
}
