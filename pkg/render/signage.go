package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Phase 21 item 38: three ways a session's title could stand on a real
// surface instead of being paint on the building. Prototypes, for Aria
// to choose from against the hover-only frame — none of them is a
// decision yet.
//
// Only sign-highway carries a board. road-sign-empty is a bare pole and
// road-sign-empty-hanging a bracket with nothing on it: "empty" in the
// kit's names means no panel at all, not a blank one. So the post and
// the plaque draw their panel the way the rooftop billboard already
// does, and only the gantry paints onto the kit's own face.
const (
	// Where sign-highway's near board sits in its sprite, and the slope
	// of that board's face. Measured off kits-z2 by the panel's own
	// colour: the top edge falls exactly 0.500 px per px across, which
	// is this projection's own gradient and is how the measurement
	// checked out.
	gantryFaceLeft, gantryFaceRight = 0.113, 0.475
	gantryFaceTop, gantryFaceBottom = 0.182, 0.525
	gantryLean                      = -ui.BoardLean
	// How far beside the building a sign stands, as a fraction of the
	// building sprite's width, and how big a drawn panel is.
	signBeside  = 0.62
	panelPadPx  = 3.0
	panelLiftPx = 2.0
)

// buildingSignage puts a session's title on the map in whichever way is
// being tried. It returns false when the caller should fall back to the
// paint-on-the-building sign that ships today.
func (g *Game) buildingSignage(screen *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point, name string) bool {
	switch g.signage {
	case city.SignageHover:
		return true
	case city.SignageGantry:
		g.signGantry(screen, cam, r, foot, name)
		return true
	case city.SignageBoard:
		g.signOnMount(screen, cam, r, foot, name, kitSignPost, 0.9)
		return true
	case city.SignagePlaque:
		g.signOnMount(screen, cam, r, foot, name, kitSignArm, 0.75)
		return true
	}
	return false
}

// beside is where a sign stands next to a building, in screen pixels:
// off the sprite's near side, on the building's own ground line. It is
// screen-space on purpose, so the sign keeps its place beside the
// building through all four headings rather than swinging around it.
func beside(r city.Rect, foot city.Point) city.Point {
	return city.Point{X: foot.X + r.Width()*signBeside, Y: foot.Y}
}

// signGantry stands a highway gantry beside the plot and paints the
// title on the kit's own board, leaning with it.
func (g *Game) signGantry(screen *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point, name string) {
	board := g.kitAt(screen, cam, kitSignGantry, 0, beside(r, foot), nil)
	if board.Area() == 0 {
		return
	}
	face := city.Rect{
		Min: city.Point{X: board.Min.X + board.Width()*gantryFaceLeft, Y: board.Min.Y + board.Height()*gantryFaceTop},
		Max: city.Point{X: board.Min.X + board.Width()*gantryFaceRight, Y: board.Min.Y + board.Height()*gantryFaceBottom},
	}
	sign, ok := ui.LayoutPlaque(name, face, gantryLean, g.faces.Measure)
	if !ok {
		return
	}
	g.sign(screen, sign, colorKitFloor)
}

// signOnMount stands one of the kit's bare mounts beside the plot and
// draws the panel it does not have, sized to the title, at head as a
// fraction of the mount's height.
func (g *Game) signOnMount(screen *ebiten.Image, cam *city.Camera, r city.Rect, foot city.Point, name, mount string, head float64) {
	post := g.kitAt(screen, cam, mount, 0, beside(r, foot), nil)
	if post.Area() == 0 || name == "" {
		return
	}
	w, h := g.faces.Measure(name, ui.Small)
	if w <= 0 || h <= 0 {
		return
	}
	scale := ui.SignFill
	if h*scale < ui.MinSignPx {
		return
	}
	panel := city.RectAt(
		post.Center().X-(w*scale)/2-panelPadPx,
		post.Min.Y+post.Height()*(1-head)-panelLiftPx,
		w*scale+panelPadPx*2, h*scale+panelPadPx,
	)
	vector.FillRect(screen, float32(panel.Min.X), float32(panel.Min.Y), float32(panel.Width()), float32(panel.Height()), colorKitKerb, false)
	vector.StrokeRect(screen, float32(panel.Min.X), float32(panel.Min.Y), float32(panel.Width()), float32(panel.Height()), 1, colorKitFloor, false)
	sign, ok := ui.LayoutPlaque(name, panel, 0, g.faces.Measure)
	if !ok {
		return
	}
	g.sign(screen, sign, colorKitFloor)
}
