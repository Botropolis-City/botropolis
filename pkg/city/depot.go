package city

import (
	"fmt"
	"sort"
	"time"

	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
)

// The depot is where packed-up sessions are kept, at the city's edge:
// one row of crates per district, oldest first (item 86, slice 3).
const (
	DepotName = "depot"
	CrateSize = 2 * Tile
	CrateGap  = Tile / 2
	RowGap    = Tile
)

// DepotRow is one district's crates in the depot.
type DepotRow struct {
	Root string
	Name string
	Rect Rect
	// Hue is the district's crate colour, an index into ContainerHues.
	Hue int
}

// buildDepot lays every packed-up session in one district along the
// south, a row per district in name order, each row oldest first. It is
// as wide as its longest row, and wraps a row that would be wider than
// the plan, whose width in pixels is widest; the district's size says
// how many rows and columns the plan must give it. With nothing in it
// there is no depot, unless keep holds one open, empty, for a truck
// still leaving it.
func buildDepot(byRoot map[string][]state.Session, widest float64, now time.Time, keep bool) *District {
	roots := make([]string, 0, len(byRoot))
	longest := 0
	for root, sessions := range byRoot {
		if len(sessions) > 0 {
			roots = append(roots, root)
			longest = max(longest, len(sessions))
		}
	}
	if len(roots) == 0 && !keep {
		return nil
	}
	width := min(widest, 2*DistrictPadding+(CrateSize+CrateGap)*float64(max(1, longest))-CrateGap)
	if len(roots) == 0 {
		return &District{Name: DepotName, Depot: true, size: Point{X: width, Y: 2*DistrictPadding + CrateSize}, at: now}
	}
	sort.Slice(roots, func(i, j int) bool { return baseName(roots[i]) < baseName(roots[j]) })
	d := &District{Name: DepotName, Depot: true}
	pitch := CrateSize + CrateGap
	right := width - DistrictPadding
	y := DistrictPadding
	for _, root := range roots {
		sessions := byRoot[root]
		sort.Slice(sessions, func(i, j int) bool {
			if !sessions[i].LastActivity.Equal(sessions[j].LastActivity) {
				return sessions[i].LastActivity.Before(sessions[j].LastActivity)
			}
			return sessions[i].ID < sessions[j].ID
		})
		row := DepotRow{Root: root, Name: baseName(root), Hue: projectHue(root)}
		x := DistrictPadding
		for i, s := range sessions {
			if x+CrateSize > right {
				x, y = DistrictPadding, y+pitch
			}
			b := newBuilding(s, i, Point{X: x, Y: y}, CrateSize, state.RungPacked)
			b.Hue = row.Hue
			if row.Rect.Area() == 0 {
				row.Rect = b.Rect
			} else {
				row.Rect = row.Rect.Union(b.Rect)
			}
			d.Buildings = append(d.Buildings, b)
			x += pitch
		}
		d.Rows = append(d.Rows, row)
		y += CrateSize + RowGap
	}
	d.size = Point{X: width, Y: y - RowGap + DistrictPadding}
	d.at = now
	return d
}

// depotCard is the depot's hover card: how many crates from how many
// districts, and each district's count and oldest.
func (d *District) depotCard() Card {
	var details []string
	for _, row := range d.Rows {
		n, oldest := 0, time.Time{}
		for _, b := range d.Buildings {
			if ProjectRoot(b.Session.CWD) != row.Root {
				continue
			}
			n++
			if oldest.IsZero() || b.Session.LastActivity.Before(oldest) {
				oldest = b.Session.LastActivity
			}
		}
		details = append(details, fmt.Sprintf("%-11s %d  oldest %s", row.Name, n, quietFor(d.at.Sub(oldest))))
	}
	return Card{
		Title:    DepotName,
		Kind:     "where packed-up sessions are kept",
		Sentence: "Unpack a crate to bring its session back to its old lot.",
		Numbers: []Number{
			{Label: "crates", Value: fmt.Sprint(len(d.Buildings))},
			{Label: "projects", Value: fmt.Sprint(len(d.Rows))},
		},
		Details: details,
	}
}

// packedLine is a crate's line on its card: when it was packed up.
func packedLine(s state.Session, now time.Time) string {
	if s.PackedAt.IsZero() {
		return "packed   up"
	}
	return "packed   " + format.Age(now.Sub(s.PackedAt)) + " ago"
}
