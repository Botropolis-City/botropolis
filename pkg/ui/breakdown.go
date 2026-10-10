package ui

import (
	"fmt"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
)

// BreakdownRow is one share: its name and what it spent.
type BreakdownRow struct {
	Label Text
	Value Text
}

// BreakdownColumn is one way of splitting the spend.
type BreakdownColumn struct {
	Head Text
	Rows []BreakdownRow
}

// BreakdownPanel is the plant's breakdown laid out: a title with the
// window's total, buttons for the three windows, the series as a
// sparkline and three columns of shares.
type BreakdownPanel struct {
	Rect    city.Rect
	Title   Text
	Windows []Button
	Current city.Window
	Spark   Sparkline
	Columns []BreakdownColumn
}

const (
	breakdownRows = 8
	sparkGrids    = 3
)

// LayoutBreakdown centres the panel in a width×height window; titles
// names sessions by id.
func LayoutBreakdown(th Theme, width, height float64, b city.Breakdown, series []float64, titles map[string]string, measure Measure) BreakdownPanel {
	grid := th.Grid()
	pad := 3 * grid
	_, bodyH := measure("", Body)
	_, titleH := measure("", Title)
	colW := 30 * grid
	w := pad + 3*colW + 2*grid + pad
	title := fmt.Sprintf("Power: %s tokens, %s in %s", format.Tokens(float64(b.Tokens)), format.USD(b.CostUSD, b.CostKnown), b.Window)
	rowStep := bodyH + grid/2
	rows := 0
	for _, col := range [][]city.Share{b.ByModel, b.ByProject, b.BySession} {
		rows = max(rows, min(len(col), breakdownRows))
	}
	h := pad + titleH + grid + (bodyH + grid) + grid + sparkGrids*grid + 2*grid + bodyH + grid/2 + rowStep*float64(rows) + pad
	p := BreakdownPanel{Rect: city.RectAt((width-w)/2, (height-h)/2, w, h), Current: b.Window}
	x0, y := p.Rect.Min.X+pad, p.Rect.Min.Y+pad
	p.Title = Text{Text: title, At: city.Point{X: x0, Y: y}, Size: Title}
	y += titleH + grid
	p.Windows = LayoutButtons(th, []string{city.LastHour.String(), city.LastDay.String(), city.LastWeek.String()}, city.Point{X: x0, Y: y}, measure)
	y += bodyH + grid + grid
	p.Spark = LayoutSparkline(th, series, city.RectAt(x0, y, w-2*pad, sparkGrids*grid))
	y += sparkGrids*grid + 2*grid
	heads := []string{"by model", "by project", "by session"}
	for i, shares := range [][]city.Share{b.ByModel, b.ByProject, b.BySession} {
		x := x0 + float64(i)*(colW+grid)
		col := BreakdownColumn{Head: Text{Text: heads[i], At: city.Point{X: x, Y: y}, Size: Small}}
		ry := y + bodyH + grid/2
		for j, share := range shares {
			if j == breakdownRows {
				break
			}
			label := share.Key
			if i == 2 {
				if t, ok := titles[share.Key]; ok && t != "" {
					label = t
				}
			}
			value := fmt.Sprintf("%s · %s", format.Tokens(float64(share.Tokens)), format.USD(share.CostUSD, share.CostKnown))
			vw, _ := measure(value, Small)
			col.Rows = append(col.Rows, BreakdownRow{
				Label: Text{Text: clipTo(label, colW-vw-grid, Small, measure), At: city.Point{X: x, Y: ry}, Size: Small},
				Value: Text{Text: value, At: city.Point{X: x + colW - vw, Y: ry}, Size: Small},
			})
			ry += rowStep
		}
		p.Columns = append(p.Columns, col)
	}
	return p
}

// HitWindow is the window whose button is under a point, if any.
func (p BreakdownPanel) HitWindow(at city.Point) (city.Window, bool) {
	for i, b := range p.Windows {
		if b.Rect.Contains(at) {
			return city.Window(i), true
		}
	}
	return 0, false
}
