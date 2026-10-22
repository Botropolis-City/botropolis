package ui

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// openingAfter is how long the city waits before it shows the opening
// panel: the pages usually go up in a few frames, and a panel that
// appears for 70 ms and goes reads as a glitch (item 159).
const openingAfter = 300 * time.Millisecond

// OpeningShown is whether a wait of waited before the city is drawn is
// long enough to show the opening panel.
func OpeningShown(waited time.Duration) bool {
	return waited >= openingAfter
}

// OpeningPages is the panel's line and fill while the atlas pages go up.
func OpeningPages(done, total int) (string, float64) {
	return "Opening the city", fraction(int64(done), int64(total))
}

// NoFill is the fill of a wait nothing can measure: the bar sweeps
// instead, so the panel never looks frozen and never claims progress.
const NoFill = -1.0

// OpeningLoad is the panel's line and fill while the first load runs;
// one that nothing has measured, as when the city reads the home itself
// or the daemon is from before the question, gets no fill.
func OpeningLoad(load state.LoadProgress) (string, float64) {
	if load.Total == 0 {
		return "Reading your sessions", NoFill
	}
	return fmt.Sprintf("Reading your sessions · %d of %d", load.Done, load.Total), fraction(int64(load.Done), int64(load.Total))
}

func fraction(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return min(max(float64(done)/float64(total), 0), 1)
}

// Opening is the panel drawn on the void while the city opens: a line
// over a bar filled as far as Fill.
type Opening struct {
	Panel     city.Rect
	Line      Text
	Bar, Fill city.Rect
	// Sweeping is whether the bar has no fill to show, so Sweep moves a
	// segment across it instead.
	Sweeping bool
}

// sweepShare is how much of the bar the sweeping segment covers.
const sweepShare = 0.2

// Sweep places the sweeping segment at phase, from 0 to 1, of its way
// across the bar; a bar with a fill is left as it is.
func (o Opening) Sweep(phase float64) Opening {
	if !o.Sweeping {
		return o
	}
	w := o.Bar.Width() * sweepShare
	x := o.Bar.Min.X + (o.Bar.Width()-w)*min(max(phase, 0), 1)
	o.Fill = city.RectAt(x, o.Bar.Min.Y, w, o.Bar.Height())
	return o
}

// LayoutOpening centres the opening panel in a width×height window.
func LayoutOpening(th Theme, width, height float64, line string, filled float64, measure Measure) Opening {
	grid := th.Grid()
	lineW, lineH := measure(line, Small)
	barW := max(lineW, 40*grid)
	barH := grid / 2
	panelW, panelH := barW+4*grid, lineH+barH+5*grid
	panel := city.RectAt((width-panelW)/2, (height-panelH)/2, panelW, panelH)
	bar := city.RectAt(panel.Min.X+2*grid, panel.Min.Y+2*grid+lineH+grid, barW, barH)
	return Opening{
		Panel:    panel,
		Line:     Text{Text: line, At: city.Point{X: panel.Min.X + 2*grid, Y: panel.Min.Y + 2*grid}, Size: Small},
		Bar:      bar,
		Fill:     city.RectAt(bar.Min.X, bar.Min.Y, bar.Width()*min(max(filled, 0), 1), barH),
		Sweeping: filled == NoFill,
	}
}
