package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// districtSign stands a board on a post at the district's front corner,
// at the map's scale, painted with its name and whose move it is in the
// move's tone (item 88). It reports false when the board is too small
// to read, and the floating plate carries the name instead.
func (g *Game) districtSign(screen *ebiten.Image, cam *city.Camera, d *city.District) bool {
	sign := d.Sign()
	foot, board, laid, ok := g.districtSignLayout(cam, d)
	if !ok {
		return false
	}
	g.queueDistrictSign(foot, board)
	g.paintDistrictSign(screen, sign, foot, board, laid)
	return true
}

// queueDistrictSign asks for the room a district's sign takes, so the
// map's labels keep off it.
func (g *Game) queueDistrictSign(foot city.Point, board city.Rect) {
	stroke := float32(g.theme.Px(1))
	g.queueSign(board, city.Rect{Min: city.Point{X: foot.X - float64(stroke), Y: board.Max.Y}, Max: city.Point{X: foot.X + float64(stroke), Y: foot.Y}})
}

// paintDistrictSign paints a district's sign as it was laid out: its
// post, its board and its words.
func (g *Game) paintDistrictSign(screen *ebiten.Image, sign city.DistrictSign, foot city.Point, board city.Rect, laid ui.DistrictSign) {
	th := g.theme
	stroke := float32(th.Px(1))
	g.strokeLine(screen, float32(foot.X), float32(foot.Y), float32(foot.X), float32(board.Max.Y), stroke*2, colorKitKerb)
	g.paint.Box(screen, float32(board.Min.X), float32(board.Min.Y), float32(board.Width()), float32(board.Height()), colorKitKerb)
	g.paint.BoxStroke(screen, float32(board.Min.X), float32(board.Min.Y), float32(board.Width()), float32(board.Height()), stroke, colorKitFloor)
	g.sign(screen, laid.Name, colorKitFloor)
	g.sign(screen, laid.Move, th.Color(ui.MoveTone(sign.Tone)))
}

// districtSignLayout is where a district's sign stands, its board, and
// its words on the board, or false when they would not stand
// ui.MinSignPx tall.
func (g *Game) districtSignLayout(cam *city.Camera, d *city.District) (city.Point, city.Rect, ui.DistrictSign, bool) {
	sign := d.Sign()
	foot := cam.WorldToScreen(sign.At)
	board := ui.DistrictSignBoard(sign.Name, sign.Move, foot, cam.Zoom, g.faces.Measure)
	laid, ok := ui.LayoutDistrictSign(sign.Name, sign.Move, board, g.faces.Measure)
	return foot, board, laid, ok
}
