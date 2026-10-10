package city

import (
	"fmt"
	"math"
	"sort"

	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
)

// The plan works in cells; a cell is one building's footprint.
const CellSize = BuildingSize

// StorageName is the storage district's name on the map.
const StorageName = "storage"

// StorageGroup is one project's parked sessions inside the storage district.
type StorageGroup struct {
	Root string
	Name string
	Rect Rect
	// Hue is the project's container colour, an index into ContainerHues.
	Hue int
}

func cellRect(b plan.Block) Rect {
	return RectAt(float64(b.Min.Col)*CellSize, float64(b.Min.Row)*CellSize, float64(b.Cols)*CellSize, float64(b.Rows)*CellSize)
}

func cellAt(c plan.Cell) Rect {
	return RectAt(float64(c.Col)*CellSize, float64(c.Row)*CellSize, CellSize, CellSize)
}

func toCell(c plan.Cell) Cell {
	return Cell{Col: c.Col, Row: c.Row}
}

func cellsNeeded(px float64) int {
	return int(math.Ceil(px / CellSize))
}

// planInput is what the plan needs from the districts: each live one's
// footprint in cells, the towers, and the storage rows once its width
// is known.
func planInput(districts []*District, towers int) plan.Input {
	in := plan.Input{Towers: towers}
	for _, d := range districts {
		in.Districts = append(in.Districts, plan.District{Root: d.Root, Cols: cellsNeeded(d.size.X), Rows: cellsNeeded(d.size.Y)})
	}
	return in
}

// buildStorage lays every parked session in one district along the
// south: projects side by side in rows of sheds, a gap between projects,
// wrapping when the row is full. width is the block's width in pixels;
// the district's size says how many rows the plan must give it.
func buildStorage(byRoot map[string][]state.Session, width float64) *District {
	roots := make([]string, 0, len(byRoot))
	for root, sessions := range byRoot {
		if len(sessions) > 0 {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		return nil
	}
	sort.Slice(roots, func(i, j int) bool { return baseName(roots[i]) < baseName(roots[j]) })
	d := &District{Name: StorageName, Storage: true}
	x, y := DistrictPadding, DistrictPadding
	right := width - DistrictPadding
	pitch := ParkedSize + ParkedGap
	for _, root := range roots {
		sessions := byRoot[root]
		sort.Slice(sessions, func(i, j int) bool {
			if !sessions[i].StartedAt.Equal(sessions[j].StartedAt) {
				return sessions[i].StartedAt.Before(sessions[j].StartedAt)
			}
			return sessions[i].ID < sessions[j].ID
		})
		need := pitch*float64(len(sessions)) - ParkedGap
		if x > DistrictPadding && x+math.Min(need, right-DistrictPadding) > right {
			x, y = DistrictPadding, y+pitch+YardGap
		}
		group := StorageGroup{Root: root, Name: baseName(root), Hue: projectHue(root)}
		for i, s := range sessions {
			if x+ParkedSize > right {
				x, y = DistrictPadding, y+pitch
			}
			b := newBuilding(s, i, Point{X: x, Y: y}, ParkedSize)
			if group.Rect.Area() == 0 {
				group.Rect = b.Rect
			} else {
				group.Rect = group.Rect.Union(b.Rect)
			}
			d.Buildings = append(d.Buildings, b)
			x += pitch
		}
		d.Groups = append(d.Groups, group)
		x += YardGap
	}
	d.size = Point{X: width, Y: y + ParkedSize + DistrictPadding}
	return d
}

// placeOn moves a district and everything in it to a block's corner.
func (d *District) placeOn(r Rect) {
	origin := r.Min
	d.Rect = r
	for _, b := range d.Buildings {
		b.Rect = Rect{Min: b.Rect.Min.Add(origin), Max: b.Rect.Max.Add(origin)}
	}
	for i := range d.Groups {
		g := &d.Groups[i]
		g.Rect = Rect{Min: g.Rect.Min.Add(origin), Max: g.Rect.Max.Add(origin)}
	}
}

// StorageCard is the storage district's hover card.
func (d *District) storageCard() Card {
	lines := []string{fmt.Sprintf("parked   %d in %s", len(d.Buildings), plural(len(d.Groups), "project"))}
	for _, g := range d.Groups {
		n := 0
		for _, b := range d.Buildings {
			if ProjectRoot(b.Session.CWD) == g.Root {
				n++
			}
		}
		lines = append(lines, fmt.Sprintf("%-11s %d  %s", g.Name, n, ContainerHues[g.Hue]))
	}
	return Card{Title: StorageName, Lines: lines}
}

// Tree is one of the plan's trees: its cell, its variant and where it
// stands, a seeded step off the cell's centre.
type Tree struct {
	Cell    Cell
	Kind    plan.TreeKind
	Variant int
	At      Point
}

// Park is a block the plan left green.
type Park struct {
	Rect Rect
}

func (Park) Card() Card {
	return Card{Title: "park", Lines: []string{"planned green: no project has this block"}}
}

// PlazaCard is the civic centre's card.
func plazaCard() Card {
	return Card{Title: "plaza", Lines: []string{"the civic centre: power plant, city hall, library"}}
}

func fountainCard() Card {
	return Card{Title: "fountain", Lines: []string{"the plaza's centrepiece; it means nothing"}}
}
