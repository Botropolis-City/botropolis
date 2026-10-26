package render

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// leftInput is one frame of the left button: whether it went down this
// frame, is held, or came up, and the modifiers held with it.
type leftInput struct {
	press, down, release bool
	// ctrl is Ctrl or Cmd: it selects districts instead (item 283).
	ctrl bool
	// pan is Shift or Alt: any drag pans, wherever it starts.
	pan bool
}

// ctrlHeld is Ctrl or Cmd held, by hand or by a script.
func (g *Game) ctrlHeld() bool {
	if g.scriptedCtrl || ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta) {
		return true
	}
	held, ok := keyByName(g.play.hold)
	if !ok || g.play.hold == "" {
		return false
	}
	switch held {
	case ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight, ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight:
		return true
	}
	return false
}

// clickSlop is how far the pointer may move between press and release
// for the two to still be a click.
const clickSlop = 3

// leftButton is the left button on the map. A press on a district's
// floor or sign picks the district up, and the selected districts with
// it when it is one of them; with Ctrl it adds the district to the
// selection or takes it out, or on open ground draws a box that selects
// every district it touches; anywhere else it pans. A press let go
// where it started is a click, measured from the press, so a pan that
// ends on a building does not select it (item 282).
func (g *Game) leftButton(cursor city.Point, in leftInput) {
	if in.press {
		g.pressAt = cursor
		switch {
		case carries(g.scene.Hover(), g.scene.Selected()):
			g.carrying = g.scene.Selected()
		case in.ctrl && !in.pan:
			if !g.scene.ToggleDistrict(cursor) {
				g.boxing = true
			}
		case g.scene.PressDistrict(cursor, in.pan):
			g.moving = true
		default:
			g.dragging, g.dragFrom = true, cursor
		}
	}
	if g.moving && in.down {
		g.scene.DragDistrict(cursor)
	}
	if in.release && g.moving {
		g.moving = false
		if dragged(g.pressAt, cursor) {
			if g.scene.DropDistrict(cursor) && g.saveState != nil {
				g.saveState(g.scene.Layout())
			}
		} else {
			g.scene.CancelDistrictDrag()
			g.act(g.scene.Click(cursor))
		}
	}
	if in.release && g.boxing {
		g.boxing = false
		g.scene.SelectBox(g.pressAt, cursor)
	}
	if g.dragging && in.down {
		g.scene.Pan(cursor.Sub(g.dragFrom))
		g.dragFrom = cursor
	}
	if in.release && g.carrying != nil {
		if drop := dropAction(g.carrying, g.scene.Hover()); drop.Kind == city.ActionMove {
			g.move(drop.SessionID, drop.Group)
		}
		g.carrying = nil
	}
	if in.release && g.dragging {
		g.dragging = false
		if !dragged(g.pressAt, cursor) {
			if hit := g.scene.Hover(); hit.District == nil && hit.Building == nil {
				g.scene.ClearSelection()
			}
			g.clickLandmark(g.scene.Hover().Landmark)
			g.act(g.scene.Click(cursor))
		}
	}
}

// selectBox is the box being drawn to select districts, in screen
// points, if one is.
func (g *Game) selectBox() (from, to city.Point, ok bool) {
	return g.pressAt, g.pointer(), g.boxing
}

func dragged(from, to city.Point) bool {
	return math.Hypot(to.X-from.X, to.Y-from.Y) > clickSlop
}
