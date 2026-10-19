package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// landmarkSprite is the screen rect a landmark's sprite is drawn into,
// standing on its plot's centre as isoLandmark stands it; the plant's
// includes the chimney rising out of its roof.
func (g *Game) landmarkSprite(cam *city.Camera, r city.Rect, name string) city.Rect {
	_, _, _, rect, ok := g.kitPlace(cam, name, 0, r.Center(), 0)
	if !ok {
		return city.Rect{}
	}
	if name == kitPlant {
		perch := plantStackPerch(r, g.kitFootprint(cam, kitPlant))
		if _, _, _, _, stack, ok := g.kitThroughPlace(cam, kitStack, perch, g.kitRoofLift(cam, kitPlant)); ok {
			rect = rect.Union(stack)
		}
	}
	return rect
}

// landmarkLabel is a landmark's name on a plate resting over the top of
// its drawn sprite, anchored on the sprite's top edge (item 99). It
// stood a sprite's height above the landmark's foot before, which is
// above the sprite by all of the sprite below its anchor.
func (g *Game) landmarkLabel(sprite city.Rect, s string) pendingLabel {
	p := g.floorPlate(city.Point{}, s)
	top := city.Point{X: sprite.Center().X, Y: sprite.Min.Y}
	p = p.Moved(city.Point{X: top.X - p.Rect.Width()/2, Y: top.Y - g.theme.Px(4) - p.Rect.Height()})
	return pendingLabel{
		mapLabel: mapLabel{Kind: labelLandmark, Rect: p.Rect, Anchor: top},
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
	g.queueLabel(g.landmarkLabel(sprite, s))
}
