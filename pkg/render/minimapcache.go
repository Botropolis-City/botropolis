package render

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// The minimap's map, the ground, the parks, a block for each district
// and a dot for each building, was drawn afresh every frame: 3,300 boxes
// on 900 districts. It changes only with the city, the minimap's box and
// the way the camera faces, so it is painted once into an image of its
// own and drawn from there until one of them changes (item 270). The
// panel under it and the view's outline over it are drawn each frame.

// minimapCache is the minimap's map as last painted, and what it was
// painted from.
type minimapCache struct {
	img        *ebiten.Image
	city       *city.City
	box        city.Rect
	heading    int
	projection city.Projection
	at         image.Point
}

// minimapMap draws the minimap's map into box: from the kept image while
// what it shows holds, and painted afresh, and kept, when it does not.
// A frame drawn straight paints it afresh onto the screen, as before.
func (g *Game) minimapMap(screen *ebiten.Image, box city.Rect, m city.Minimap, c *city.City, all city.Rect) {
	if g.noStill {
		g.paintMinimapMap(screen, image.Point{}, m, c, all)
		return
	}
	cam := g.scene.Camera()
	k := &g.mini
	if k.img == nil || k.city != c || k.box != box || k.heading != cam.Heading || k.projection != cam.Projection {
		at := image.Point{X: int(math.Floor(box.Min.X)) - 2, Y: int(math.Floor(box.Min.Y)) - 2}
		size := image.Point{X: int(math.Ceil(box.Max.X)) + 2 - at.X, Y: int(math.Ceil(box.Max.Y)) + 2 - at.Y}
		if k.img == nil || k.img.Bounds().Size() != size {
			if k.img != nil {
				k.img.Deallocate()
			}
			k.img = ebiten.NewImageWithOptions(image.Rectangle{Max: size}, &ebiten.NewImageOptions{Unmanaged: true})
		} else {
			k.img.Clear()
		}
		g.paintMinimapMap(k.img, at, m, c, all)
		k.city, k.box, k.heading, k.projection, k.at = c, box, cam.Heading, cam.Projection, at
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(k.at.X), float64(k.at.Y))
	screen.DrawImage(k.img, op)
}

// paintMinimapMap paints the minimap's map onto dst, whose origin stands
// at origin on the screen.
func (g *Game) paintMinimapMap(dst *ebiten.Image, origin image.Point, m city.Minimap, c *city.City, all city.Rect) {
	g.minimapPaints++
	ox, oy := float32(origin.X), float32(origin.Y)
	paint := g.paint.Box
	if g.minimapPainter != nil {
		paint = g.minimapPainter
	}
	box := func(r city.Rect, col color.Color) {
		paint(dst, float32(r.Min.X)-ox, float32(r.Min.Y)-oy, float32(math.Max(1, r.Width())), float32(math.Max(1, r.Height())), col)
	}
	box(all, colorGroundNight)
	for _, park := range c.Parks {
		box(m.ProjectRect(park.Rect), colorMapPark)
	}
	if c.Plaza.Area() > 0 {
		box(m.ProjectRect(c.Plaza), colorMapPlaza)
	}
	for _, d := range c.Districts {
		box(m.ProjectRect(d.Rect), colorMapDistHi)
		for _, b := range d.Buildings {
			if b.Blinds {
				continue
			}
			p := m.Project(b.Rect.Center())
			paint(dst, float32(p.X-1)-ox, float32(p.Y-1)-oy, 2, 2, blockColor(b, 0))
		}
	}
}
