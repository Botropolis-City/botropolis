package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// landmarkSprite is the screen rect a landmark's sprite is drawn into,
// standing on its plot's centre as isoLandmark stands it; the plant's
// includes the chimney rising out of its roof.
func (g *Game) landmarkSprite(cam *city.Camera, r city.Rect, name string) city.Rect {
	_, _, _, rect, ok := g.kitPlace(cam, name, landmarkTurn, r.Center(), 0)
	if !ok {
		return city.Rect{}
	}
	if name == kitPlant {
		perch := landmarkPerch(r, g.kitFootprintTurned(cam, kitPlant, landmarkTurn))
		if _, _, _, _, stack, ok := g.kitThroughPlace(cam, kitStack, perch, g.kitRoofLiftTurned(cam, kitPlant, landmarkTurn)); ok {
			rect = rect.Union(stack)
		}
	}
	return rect
}

// landmarkLabel is a landmark's name on a plate resting on its roof
// (item 99; Aria, 2026-10-19: "right on/near the roof"). It stood a
// sprite's height above the landmark's foot before, which is above the
// sprite by all of the sprite below its anchor, and so on whatever
// stood behind it.
func (g *Game) landmarkLabel(roof city.Point, s string) pendingLabel {
	p := g.floorPlate(city.Point{}, s)
	p = p.Moved(city.Point{X: roof.X - p.Rect.Width()/2, Y: roof.Y - p.Rect.Height()/2})
	return pendingLabel{
		mapLabel: mapLabel{Kind: labelLandmark, Rect: p.Rect, Anchor: roof},
		draw: func(screen *ebiten.Image, at city.Rect) {
			g.plate(screen, p.Moved(at.Min), s, colorDim)
		},
		ink: g.theme.Palette.Panel,
	}
}

// queueLandmarkLabel asks for a landmark's label, if its sprite is drawn.
func (g *Game) queueLandmarkLabel(cam *city.Camera, r city.Rect, name, s string) {
	sprite := g.landmarkSprite(cam, r, name)
	if sprite.Area() == 0 {
		return
	}
	g.queueLabel(g.landmarkLabel(g.landmarkRoof(cam, r, name), s))
}

// landmarkRoof is the point on a landmark's roof plane over its plot's
// centre, where the chimney's flashing meets the plant's roof too.
func (g *Game) landmarkRoof(cam *city.Camera, r city.Rect, name string) city.Point {
	foot := cam.WorldToScreen(r.Center())
	return city.Point{X: foot.X, Y: foot.Y - g.kitRoofLiftTurned(cam, name, landmarkTurn)}
}
