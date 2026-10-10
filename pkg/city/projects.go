package city

import (
	"path/filepath"
	"sort"

	"botropolis.city/botropolis/pkg/state"
)

// ProjectRow is one project as the sidebar lists it: its counts, its
// marks, and its sessions by urgency with parked ones last.
type ProjectRow struct {
	Root     string
	Name     string
	Live     int
	NeedsYou int
	Starred  bool
	Sessions []*Building
}

// Projects lists every project on the map, starred ones first and the
// rest by name; a project's parked sessions come from storage.
func (c *City) Projects(layout *Layout) []ProjectRow {
	byRoot := map[string]*ProjectRow{}
	var roots []string
	add := func(b *Building) {
		root := ProjectRoot(b.Session.CWD)
		row, ok := byRoot[root]
		if !ok {
			row = &ProjectRow{Root: root, Name: filepath.Base(root), Starred: layout != nil && layout.IsStarred(root)}
			byRoot[root] = row
			roots = append(roots, root)
		}
		row.Sessions = append(row.Sessions, b)
		if state.Live(b.Session.State) {
			row.Live++
		}
		if b.Session.State == state.NeedsYou {
			row.NeedsYou++
		}
	}
	for _, d := range c.Districts {
		for _, b := range d.Buildings {
			add(b)
		}
	}
	rows := make([]ProjectRow, 0, len(roots))
	for _, root := range roots {
		row := byRoot[root]
		sort.SliceStable(row.Sessions, func(i, j int) bool {
			a, b := row.Sessions[i].Session, row.Sessions[j].Session
			if state.Rank(a.State) != state.Rank(b.State) {
				return state.Rank(a.State) < state.Rank(b.State)
			}
			return a.LastActivity.After(b.LastActivity)
		})
		rows = append(rows, *row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Starred != rows[j].Starred {
			return rows[i].Starred
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}
