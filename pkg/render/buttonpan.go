package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"botropolis.city/botropolis/pkg/city"
)

// panButtons pan the map from wherever they are pressed, and never pick
// up a district or a building: the left button's drag does both (item 279).
var panButtons = []ebiten.MouseButton{ebiten.MouseButtonRight, ebiten.MouseButtonMiddle}

type mouseInput interface {
	JustPressed(ebiten.MouseButton) bool
	Pressed(ebiten.MouseButton) bool
	JustReleased(ebiten.MouseButton) bool
}

type ebitenMouse struct{}

func (ebitenMouse) JustPressed(b ebiten.MouseButton) bool {
	return inpututil.IsMouseButtonJustPressed(b)
}
func (ebitenMouse) Pressed(b ebiten.MouseButton) bool { return ebiten.IsMouseButtonPressed(b) }
func (ebitenMouse) JustReleased(b ebiten.MouseButton) bool {
	return inpututil.IsMouseButtonJustReleased(b)
}

// buttonPan pans the map while the right or middle button is held. A
// city under a scrim takes no pan, as it takes no hover (item 264),
// except while sharing, which hides every panel to frame the shot.
func (g *Game) buttonPan(cursor city.Point, in mouseInput) {
	for _, b := range panButtons {
		if in.JustPressed(b) && !g.buttonPanning && (g.share != nil || !g.cityCovered()) {
			g.buttonPanning, g.buttonPanWith, g.buttonPanFrom = true, b, cursor
		}
	}
	if !g.buttonPanning {
		return
	}
	if in.Pressed(g.buttonPanWith) || in.JustReleased(g.buttonPanWith) {
		g.scene.Pan(cursor.Sub(g.buttonPanFrom))
		g.buttonPanFrom = cursor
	}
	if !in.Pressed(g.buttonPanWith) {
		g.buttonPanning = false
	}
}
