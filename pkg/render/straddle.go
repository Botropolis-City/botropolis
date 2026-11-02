package render

import (
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// hullPiece is a slice of a hull in the depth sort: the world footprint
// it stands on and the screen columns of the sprite it draws.
type hullPiece struct {
	foot     city.Rect
	from, to float64
}

// hullDraw is a boat to draw: its piece, turn, where it floats, the way
// its hull lies in the world, and its size.
type hullDraw struct {
	piece  string
	turn   int
	at     city.Point
	along  city.Point
	extent float64
	hit    func(city.Rect)
}

// kitSizedPlace works out where kitSized would draw a piece, without
// drawing it.
func (g *Game) kitSizedPlace(cam *city.Camera, name string, turn int, at city.Point, target float64) (img pagedSprite, origin city.Point, scale float64, rect city.Rect, ok bool) {
	if g.kits == nil || target <= 0 {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	sprite, found := atlas.Sprite(name, cam.Heading-turn)
	if !found {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	longest := math.Max(float64(sprite.Rect.Dx()), float64(sprite.Rect.Dy()))
	if longest <= 0 {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	scale = cam.Zoom / atlas.Zoom * kitShrink(target, longest, atlas.Zoom)
	foot := cam.WorldToScreen(at)
	origin = city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	if sprite.Page < len(atlas.pages) {
		img = spriteOn(atlas.pages[sprite.Page], sprite.Rect)
	}
	return img, origin, scale, city.RectAt(origin.X, origin.Y, float64(sprite.Rect.Dx())*scale, float64(sprite.Rect.Dy())*scale), true
}

// hullLength is how long a hull lying along a direction is in the
// world, read back from the screen columns its sprite is drawn across.
func hullLength(cam *city.Camera, at, along city.Point, drawn city.Rect) float64 {
	step := cam.WorldToScreen(at.Add(along)).X - cam.WorldToScreen(at).X
	if step == 0 {
		return 0
	}
	return (drawn.Max.X - drawn.Min.X) / math.Abs(step)
}

// hullCuts is a hull cut where the edges of every raised deck it passes
// under cross its centreline (item 259): its pieces from one end to the
// other, each standing on its own stretch of the hull. A hull clear of
// every deck edge is one piece.
//
// A sprite carries no depth, so the cut is a screen column: where the
// edge crosses the centreline it is right at every height, and off by
// at most half the beam at the hull's sides.
func hullCuts(cam *city.Camera, at, along city.Point, cols [2]float64, bridges []city.Bridge) []hullPiece {
	sx := func(t float64) float64 { return cam.WorldToScreen(at.Add(along.Scale(t))).X }
	step := sx(1) - sx(0)
	if step == 0 {
		return nil
	}
	ta, tb := (cols[0]-sx(0))/step, (cols[1]-sx(0))/step
	if ta > tb {
		ta, tb = tb, ta
	}
	ts := []float64{ta}
	for _, b := range bridges {
		if len(b.Deck) == 0 || along.Y == 0 || at.X < min(b.Land.X, b.Far.X) || at.X > max(b.Land.X, b.Far.X) {
			continue
		}
		rows := b.Abutment.Rect()
		for _, edge := range []float64{rows.Min.Y, rows.Max.Y} {
			if t := (edge - at.Y) / along.Y; t > ta && t < tb {
				ts = append(ts, t)
			}
		}
	}
	ts = append(ts, tb)
	slices.Sort(ts)
	half := city.Tile / 2
	across := city.Point{X: math.Abs(along.Y) * half, Y: math.Abs(along.X) * half}
	pieces := make([]hullPiece, 0, len(ts)-1)
	for i := 1; i < len(ts); i++ {
		p0, p1 := at.Add(along.Scale(ts[i-1])), at.Add(along.Scale(ts[i]))
		foot := city.Rect{
			Min: city.Point{X: min(p0.X, p1.X) - across.X, Y: min(p0.Y, p1.Y) - across.Y},
			Max: city.Point{X: max(p0.X, p1.X) + across.X, Y: max(p0.Y, p1.Y) + across.Y},
		}
		from, to := sx(ts[i-1]), sx(ts[i])
		if i == 1 {
			from = cols[0]
			if step < 0 {
				from = cols[1]
			}
		}
		if i == len(ts)-1 {
			to = cols[1]
			if step < 0 {
				to = cols[0]
			}
		}
		pieces = append(pieces, hullPiece{foot: foot, from: min(from, to), to: max(from, to)})
	}
	return pieces
}

// pieceColumns is the source columns, of width, a piece drawn from
// screen column from to to takes, for a sprite drawn at origin and
// scale. Neighbouring pieces round their shared cut the same way, so
// together they take every column once.
func pieceColumns(origin, scale float64, width int, from, to float64) (lo, hi int) {
	col := func(x float64) int { return max(0, min(width, int(math.Round((x-origin)/scale)))) }
	return col(from), col(to)
}

// hullItems is a boat in the depth sort: one drawable, or one per piece
// when it straddles a deck's edge (item 259).
func (g *Game) hullItems(screen *ebiten.Image, cam *city.Camera, h hullDraw, bridges []city.Bridge) []drawable {
	img, origin, scale, rect, ok := g.kitSizedPlace(cam, h.piece, h.turn, h.at, h.extent)
	if !ok {
		return nil
	}
	whole := func() {
		r := g.kitSized(screen, cam, h.piece, h.turn, h.at, h.extent, nil)
		if h.hit != nil {
			h.hit(r)
		}
	}
	cuts := hullCuts(cam, h.at, h.along, [2]float64{rect.Min.X, rect.Max.X}, bridges)
	if len(cuts) < 2 {
		d := standingAt(cam, h.at, whole)
		d.moves = true
		return []drawable{d}
	}
	width := int(math.Round((rect.Max.X - rect.Min.X) / scale))
	items := make([]drawable, 0, len(cuts))
	for i, p := range cuts {
		lo, hi := pieceColumns(origin.X, scale, width, p.from, p.to)
		first := i == 0
		d := footprintAt(cam, p.foot, func() {
			if first && h.hit != nil {
				h.hit(rect)
			}
			if hi <= lo || img.img == nil {
				return
			}
			b := img.img.Bounds()
			sub := image.Rect(b.Min.X+lo, b.Min.Y, b.Min.X+hi, b.Max.Y)
			g.drawSprite(screen, spriteOn(img.page, sub), city.Point{X: origin.X + float64(lo)*scale, Y: origin.Y}, scale, nil)
		})
		d.moves = true
		items = append(items, d)
	}
	return items
}
