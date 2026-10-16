package city

import (
	"math"

	"botropolis.city/botropolis/pkg/plan"
)

// The plan works in cells; a cell is one building's footprint.
const CellSize = BuildingSize

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

// planInput is what the plan needs from the districts: each one's
// footprint in cells, and the towers.
func planInput(districts []*District, towers int) plan.Input {
	in := plan.Input{Towers: towers}
	for _, d := range districts {
		in.Districts = append(in.Districts, plan.District{Root: d.Root, Cols: cellsNeeded(d.size.X), Rows: cellsNeeded(d.size.Y)})
	}
	return in
}

// placeOn moves a district and everything in it to a block's corner.
func (d *District) placeOn(r Rect) {
	origin := r.Min
	d.Rect = r
	for _, b := range d.Buildings {
		b.Rect = Rect{Min: b.Rect.Min.Add(origin), Max: b.Rect.Max.Add(origin)}
	}
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

// PlazaCard is the civic centre's card.
func plazaCard() Card {
	return Card{Title: "plaza", Lines: []string{"the civic centre: power plant, city hall, library"}}
}

func fountainCard() Card {
	return Card{Title: "fountain", Lines: []string{"the plaza's centrepiece; it means nothing"}}
}
