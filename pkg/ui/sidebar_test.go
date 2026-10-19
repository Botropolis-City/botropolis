package ui_test

import (
	"fmt"
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
		untitled := &city.Building{Session: state.Session{ID: "5c7c6cd3-9e32-4c60-871d-0366014d", State: state.Resting}}
		rows := []city.ProjectRow{{Root: "/p/x", Name: "x", Sessions: []*city.Building{untitled}}}
		sb := ui.LayoutSidebar(th, 40, 600, rows, nil, "", 0, false, measure7)
		require.Len(t, sb.Rows, 2)

		t.Run("its row should show the short id", func(t *testing.T) {
			assert.Equal(t, "5c7c6cd3", sb.Rows[1].Label.Text)
		})
	})
}

func TestLayoutSidebar(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when the sidebar is laid out", func(t *testing.T) {
		sb := ui.LayoutSidebar(th, 32, 568, someProjects(), []string{"/p/old"}, "d", 0, false, measure7)

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
		sb := ui.LayoutSidebar(th, 32, 32+8+24*4+8, someProjects(), nil, "", 0, false, measure7)

		t.Run("it should mark the rest below", func(t *testing.T) {
			assert.Equal(t, fmt.Sprintf("%d more below", 5-len(sb.Rows)), sb.Below.Text)
		})
	})

	t.Run("when there are more rows than fit", func(t *testing.T) {
		projects := manyProjects(10)
		hidden := []string{"/p/old", "/p/older"}
		total := len(projects)*3 + 1 + len(hidden)
		lay := func(skip int, selected string, reveal bool) ui.Sidebar {
			return ui.LayoutSidebar(th, 32, 332, projects, hidden, selected, skip, reveal, measure7)
		}
		shows := func(sb ui.Sidebar, id string) bool {
			for _, r := range sb.Rows {
				if r.ID == id {
					return true
				}
			}
			return false
		}

		t.Run("and nothing is scrolled", func(t *testing.T) {
			sb := lay(0, "", false)

			t.Run("it should start at the first row", func(t *testing.T) {
				assert.Equal(t, "/p/p00", sb.Rows[0].Root)
			})

			t.Run("it should mark how many are below", func(t *testing.T) {
				assert.Equal(t, fmt.Sprintf("%d more below", total-len(sb.Rows)), sb.Below.Text)
			})

			t.Run("it should mark nothing above", func(t *testing.T) {
				assert.Empty(t, sb.Above.Text)
			})

			t.Run("it should page by no more rows than it shows", func(t *testing.T) {
				assert.LessOrEqual(t, sb.Page, len(sb.Rows))
			})

			t.Run("it should page by as many rows as fit between both marks", func(t *testing.T) {
				assert.Equal(t, len(lay(5, "", false).Rows), sb.Page)
			})

			t.Run("it should keep every row inside the panel, clear of the marks", func(t *testing.T) {
				var out []city.Rect
				for _, r := range sb.Rows {
					if sb.Rect.Union(r.Rect) != sb.Rect || r.Rect.Max.Y > sb.Below.At.Y {
						out = append(out, r.Rect)
					}
				}
				assert.Empty(t, out, "below mark at %v", sb.Below.At)
			})
		})

		t.Run("and it is scrolled", func(t *testing.T) {
			sb := lay(5, "", false)

			t.Run("it should show the rows after the fold", func(t *testing.T) {
				assert.Equal(t, "p01-b", sb.Rows[0].ID)
			})

			t.Run("it should mark how many are above", func(t *testing.T) {
				assert.Equal(t, "5 more above", sb.Above.Text)
			})

			t.Run("it should keep its rows clear of the mark above", func(t *testing.T) {
				assert.GreaterOrEqual(t, sb.Rows[0].Rect.Min.Y, sb.Above.At.Y+16)
			})
		})

		t.Run("and it is scrolled past the end", func(t *testing.T) {
			sb := lay(1000, "", false)

			t.Run("it should stop at the last row", func(t *testing.T) {
				assert.Equal(t, sb.Max, sb.Skip)
			})
		})

		t.Run("and it is scrolled as far as it goes", func(t *testing.T) {
			sb := lay(1000, "", false)

			t.Run("it should end with the hidden section's last row", func(t *testing.T) {
				last := sb.Rows[len(sb.Rows)-1]
				assert.Equal(t, [2]any{ui.RowHidden, "/p/older"}, [2]any{last.Kind, last.Root})
			})

			t.Run("it should mark nothing below", func(t *testing.T) {
				assert.Empty(t, sb.Below.Text)
			})
		})

		t.Run("and the selection is below the fold", func(t *testing.T) {
			t.Run("and it is revealed", func(t *testing.T) {
				t.Run("it should scroll so the selected row is shown", func(t *testing.T) {
					assert.True(t, shows(lay(0, "p09-b", true), "p09-b"))
				})
			})

			t.Run("and it is not revealed", func(t *testing.T) {
				t.Run("it should leave the scroll where it is", func(t *testing.T) {
					assert.Equal(t, 0, lay(0, "p09-b", false).Skip)
				})
			})
		})

		t.Run("and the selection is above the fold and revealed", func(t *testing.T) {
			t.Run("it should scroll up so the selected row is shown", func(t *testing.T) {
				assert.True(t, shows(lay(1000, "p00-a", true), "p00-a"))
			})
		})

		t.Run("and the selection is in view and revealed", func(t *testing.T) {
			t.Run("it should not move", func(t *testing.T) {
				assert.Equal(t, 3, lay(3, "p01-a", true).Skip)
			})
		})
	})
}

func manyProjects(n int) []city.ProjectRow {
	var rows []city.ProjectRow
	for i := range n {
		name := fmt.Sprintf("p%02d", i)
		rows = append(rows, city.ProjectRow{Root: "/p/" + name, Name: name, Live: 2, Sessions: []*city.Building{
			{Session: state.Session{ID: name + "-a", Title: name + " a", State: state.Working}},
			{Session: state.Session{ID: name + "-b", Title: name + " b", State: state.Resting}},
		}})
	}
	return rows
}
