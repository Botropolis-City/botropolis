package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

func someProjects() []city.ProjectRow {
	return []city.ProjectRow{
		{Root: "/p/cinders", Name: "cinders", Live: 2, NeedsYou: 1, Starred: true, Sessions: []*city.Building{
			{Session: state.Session{ID: "c", Title: "Fix the CI queue", State: state.NeedsYou}},
			{Session: state.Session{ID: "d", Title: "Scaffold", State: state.Working}},
		}},
		{Root: "/p/app", Name: "app", Live: 1, Sessions: []*city.Building{
			{Session: state.Session{ID: "e", Title: "", State: state.Unattended}},
		}},
	}
}

func TestUntitledSidebarRow(t *testing.T) {
	t.Run("when a session has no title", func(t *testing.T) {
		th := ui.NewTheme(1)
		untitled := &city.Building{Session: state.Session{ID: "5c7c6cd3-9e32-4c60-871d-0366014d", State: state.Parked}}
		rows := []city.ProjectRow{{Root: "/p/x", Name: "x", Sessions: []*city.Building{untitled}}}
		sb := ui.LayoutSidebar(th, 40, 600, rows, nil, "", measure7)
		require.Len(t, sb.Rows, 2)

		t.Run("its row should show the short id", func(t *testing.T) {
			assert.Equal(t, "5c7c6cd3", sb.Rows[1].Label.Text)
		})
	})
}

func TestLayoutSidebar(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when the sidebar is laid out", func(t *testing.T) {
		sb := ui.LayoutSidebar(th, 32, 568, someProjects(), []string{"/p/old"}, "d", measure7)

		t.Run("it should run from the strip to the footer down the left", func(t *testing.T) {
			assert.Equal(t, city.RectAt(0, 32, 288, 536), sb.Rect)
		})

		t.Run("it should lead each project with its row, then its sessions", func(t *testing.T) {
			require.GreaterOrEqual(t, len(sb.Rows), 5)
			assert.Equal(t, []ui.RowKind{ui.RowProject, ui.RowSession, ui.RowSession, ui.RowProject, ui.RowSession}, []ui.RowKind{sb.Rows[0].Kind, sb.Rows[1].Kind, sb.Rows[2].Kind, sb.Rows[3].Kind, sb.Rows[4].Kind})
		})

		t.Run("it should star a starred project's name", func(t *testing.T) {
			assert.Equal(t, "★ cinders", sb.Rows[0].Label.Text)
		})

		t.Run("it should say what needs you on the project row", func(t *testing.T) {
			assert.Equal(t, "1 need you · 2 live", sb.Rows[0].Detail.Text)
		})

		t.Run("it should dot a session with its state's tone", func(t *testing.T) {
			assert.Equal(t, ui.ToneNeedsYou, sb.Rows[1].Tone)
		})

		t.Run("it should fall back to the id for an untitled session", func(t *testing.T) {
			assert.Equal(t, "e", sb.Rows[4].Label.Text)
		})

		t.Run("it should flag the selected session", func(t *testing.T) {
			assert.True(t, sb.Rows[2].Selected)
			assert.False(t, sb.Rows[1].Selected)
		})

		t.Run("it should stack rows one under the other", func(t *testing.T) {
			assert.Equal(t, sb.Rows[0].Rect.Max.Y, sb.Rows[1].Rect.Min.Y)
		})

		t.Run("it should end with the hidden projects", func(t *testing.T) {
			last := sb.Rows[len(sb.Rows)-1]
			assert.Equal(t, ui.RowHidden, last.Kind)
			assert.Equal(t, "/p/old", last.Root)
		})

		t.Run("it should find a row under a point", func(t *testing.T) {
			row, ok := sb.Hit(sb.Rows[2].Rect.Center())
			require.True(t, ok)
			assert.Equal(t, "d", row.ID)
		})

		t.Run("it should find nothing beside the panel", func(t *testing.T) {
			_, ok := sb.Hit(city.Point{X: 500, Y: 100})
			assert.False(t, ok)
		})
	})

	t.Run("when the panel is too short for every row", func(t *testing.T) {
		sb := ui.LayoutSidebar(th, 32, 32+8+24*2+8, someProjects(), nil, "", measure7)

		t.Run("it should drop the rows past the bottom", func(t *testing.T) {
			assert.Len(t, sb.Rows, 2)
		})
	})
}
