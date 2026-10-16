package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

const (
	// districtSignScale is the board's paint at zoom 1, as a share of
	// the chrome's small type, so every board's lettering is one size;
	// the post stands districtPostRatio of the board's height, tall
	// enough to carry the board over a session's plaque below it.
	districtSignScale = 1.3
	districtPostRatio = 2.2
)

// districtSign stands a board on a post at the district's front corner,
// at the map's scale, painted with its name and whose move it is in the
// move's tone (item 88). It reports false when the board is too small
// to read, and the floating plate carries the name instead.
func (g *Game) districtSign(screen *ebiten.Image, cam *city.Camera, d *city.District) bool {
	sign := d.Sign()
	foot := cam.WorldToScreen(sign.At)
	nw, lineH := g.faces.Measure(sign.Name, ui.Small)
	mw, _ := g.faces.Measure(sign.Move, ui.Small)
	scale := cam.Zoom * districtSignScale
	w, h := max(nw, mw)*scale/ui.SignFill, 2*lineH*scale/ui.SignFill
	post := h * districtPostRatio
	board := city.RectAt(foot.X-w/2, foot.Y-post-h, w, h)
	laid, ok := ui.LayoutDistrictSign(sign.Name, sign.Move, board, g.faces.Measure)
	if !ok {
		return false
	}
	th := g.theme
	stroke := float32(th.Px(1))
	vector.StrokeLine(screen, float32(foot.X), float32(foot.Y), float32(foot.X), float32(board.Max.Y), stroke*2, colorKitKerb, true)
	vector.FillRect(screen, float32(board.Min.X), float32(board.Min.Y), float32(board.Width()), float32(board.Height()), colorKitKerb, false)
	vector.StrokeRect(screen, float32(board.Min.X), float32(board.Min.Y), float32(board.Width()), float32(board.Height()), stroke, colorKitFloor, false)
	g.sign(screen, laid.Name, colorKitFloor)
	g.sign(screen, laid.Move, th.Color(ui.MoveTone(sign.Tone)))
	return true
}
