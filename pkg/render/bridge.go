package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// The bridge off the island. Item 132: the rail stays on the ground and
// the road goes over it, as in the real world — the landing cell is the
// abutment, the road climbs its outer half as a wedge, and the deck
// stands on piers from there out to sea, high enough that the tallest
// train piece passes under it. The deck's surface is the roads kit's
// straight piece; its girders, piers, railings and the abutment are
// geometry drawn here (design/art-sources.md).

const (
	// deckPiece is the road the deck is laid with, east-west.
	deckPiece = "city-kit-roads/road-straight"
	// deckThickness is the girder under the road, and railHeight the
	// railing over it, in screen pixels at zoom 1.
	deckThickness = 6.0
	railHeight    = 5.0
	// headroom is the clear gap over the tallest train piece's roof,
	// as a share of its height.
	headroom = 0.15
	// flatDeck is the clearance when there is no atlas to measure the
	// train against.
	flatDeck = 44.0
)

var (
	colorPier     = color.NRGBA{0x5a, 0x5e, 0x66, 0xff}
	colorGirder   = color.NRGBA{0x86, 0x89, 0x90, 0xff}
	colorAbutment = color.NRGBA{0x9c, 0x98, 0x8e, 0xff}
	colorRailing  = color.NRGBA{0xd4, 0xd6, 0xda, 0xff}
)

// deckUnderside is how high the bottom of the deck's girder stands over
// the ground on screen: clear of the tallest train piece's roof at this
// camera.
func (g *Game) deckUnderside(cam *city.Camera) float64 {
	roof := 0.0
	for _, piece := range append(append([]string{}, kitLocos...), kitWagons...) {
		for _, turn := range []int{0, 90, 180, 270} {
			roof = math.Max(roof, g.kitRoofLiftTurned(cam, piece, turn))
		}
	}
	if roof == 0 {
		return flatDeck * cam.Zoom
	}
	return roof * (1 + headroom)
}

// deckLift is how high the road on the deck stands over the ground.
func (g *Game) deckLift(cam *city.Camera) float64 {
	return g.deckUnderside(cam) + deckThickness*cam.Zoom
}

// Levels in the depth sort: a raised thing draws after a ground thing
// whose footprint it overlaps, and a thing on the deck after the deck.
const (
	levelGround = iota
	levelDeck
	levelOnDeck
)

// abutmentRect is the landing cell's outer half, where the road climbs.
func abutmentRect(b city.Bridge) city.Rect {
	r := b.Abutment.Rect()
	if b.Far.X < b.Land.X {
		r.Max.X = b.Land.X
	} else {
		r.Min.X = b.Land.X
	}
	return r
}

// bridgeItems is the bridge in the depth sort: the abutment's wedge and
// one span per deck cell, all raised.
func bridgeItems(cam *city.Camera, b city.Bridge, draw func(cell city.Cell, abutment bool) func()) []drawable {
	if len(b.Deck) == 0 {
		return nil
	}
	items := make([]drawable, 0, len(b.Deck)+1)
	abutment := footprintAt(cam, abutmentRect(b), draw(b.Abutment, true))
	abutment.level = levelDeck
	items = append(items, abutment)
	for _, cell := range b.Deck {
		span := footprintAt(cam, cell.Rect(), draw(cell, false))
		span.level = levelDeck
		items = append(items, span)
	}
	return items
}

// carriageAt is a train piece in the depth sort, on the ground.
func carriageAt(cam *city.Camera, at city.Point, draw func()) drawable {
	return standingAt(cam, at, draw)
}

// deliveryAt is a delivery truck in the depth sort, on the deck once it
// has started to climb.
func deliveryAt(cam *city.Camera, b city.Bridge, at city.Point, draw func()) drawable {
	d := standingAt(cam, at, draw)
	if b.Rise(at) > 0 {
		d.level = levelOnDeck
	}
	return d
}

// raised is a world point on screen, lift pixels above the ground.
func raised(cam *city.Camera, p city.Point, lift float64) city.Point {
	s := cam.WorldToScreen(p)
	s.Y -= lift
	return s
}

func fillScreen(screen *ebiten.Image, pts []city.Point, c color.NRGBA) {
	var path vector.Path
	for i, p := range pts {
		if i == 0 {
			path.MoveTo(float32(p.X), float32(p.Y))
		} else {
			path.LineTo(float32(p.X), float32(p.Y))
		}
	}
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(screen, &path, nil, op)
}

func strokeScreen(screen *ebiten.Image, a, b city.Point, width float64, c color.NRGBA) {
	vector.StrokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(width), c, true)
}

// nearer is whether world point a is nearer the camera than b.
func nearer(cam *city.Camera, a, b city.Point) bool {
	ta, tb := cam.TurnPoint(a), cam.TurnPoint(b)
	return ta.X+ta.Y > tb.X+tb.Y
}

// sides is a cell's two edges along the bridge, far one first.
func sides(cam *city.Camera, r city.Rect) [2][2]city.Point {
	north := [2]city.Point{{X: r.Min.X, Y: r.Min.Y}, {X: r.Max.X, Y: r.Min.Y}}
	south := [2]city.Point{{X: r.Min.X, Y: r.Max.Y}, {X: r.Max.X, Y: r.Max.Y}}
	if nearer(cam, north[0], south[0]) {
		return [2][2]city.Point{south, north}
	}
	return [2][2]city.Point{north, south}
}

// railing runs along an edge of the road at a height, with a post at
// each end and every quarter of the way.
func railing(screen *ebiten.Image, cam *city.Camera, a, b city.Point, liftA, liftB float64) {
	up := railHeight * cam.Zoom
	strokeScreen(screen, raised(cam, a, liftA+up), raised(cam, b, liftB+up), math.Max(1, cam.Zoom), colorRailing)
	for i := 0; i <= 4; i++ {
		f := float64(i) / 4
		p := lerpPoint(a, b, f)
		lift := liftA + (liftB-liftA)*f
		strokeScreen(screen, raised(cam, p, lift), raised(cam, p, lift+up), math.Max(1, 0.8*cam.Zoom), colorRailing)
	}
}

// drawAbutment is the wedge the road climbs on the landing cell's outer
// half: its sides, the wall facing the rail, and the ramp on top.
func (g *Game) drawAbutment(screen *ebiten.Image, cam *city.Camera, b city.Bridge) {
	r := abutmentRect(b)
	top := g.deckLift(cam)
	inner, outer := r.Min.X, r.Max.X
	if b.Far.X < b.Land.X {
		inner, outer = r.Max.X, r.Min.X
	}
	edge := func(y float64) (in, out city.Point) {
		return city.Point{X: inner, Y: y}, city.Point{X: outer, Y: y}
	}
	far, near := r.Min.Y, r.Max.Y
	if nearer(cam, city.Point{X: outer, Y: r.Min.Y}, city.Point{X: outer, Y: r.Max.Y}) {
		far, near = near, far
	}
	side := func(y float64) {
		in, out := edge(y)
		fillScreen(screen, []city.Point{raised(cam, in, 0), raised(cam, out, 0), raised(cam, out, top)}, colorAbutment)
	}
	side(far)
	fi, fo := edge(far)
	ni, no := edge(near)
	fillScreen(screen, []city.Point{raised(cam, fo, 0), raised(cam, no, 0), raised(cam, no, top), raised(cam, fo, top)}, colorAbutment)
	fillScreen(screen, []city.Point{raised(cam, fi, 0), raised(cam, ni, 0), raised(cam, no, top), raised(cam, fo, top)}, colorStreet)
	railing(screen, cam, fi, fo, 0, top)
	side(near)
	railing(screen, cam, ni, no, 0, top)
}

// drawSpan is one cell of the deck: the pier at its outer edge, the
// girders, the road and its railings.
func (g *Game) drawSpan(screen *ebiten.Image, cam *city.Camera, b city.Bridge, cell city.Cell) {
	r := cell.Rect()
	top := g.deckLift(cam)
	under := g.deckUnderside(cam)
	out := r.Max.X
	if b.Far.X < b.Land.X {
		out = r.Min.X
	}
	inset := 0.2 * (r.Max.Y - r.Min.Y)
	pa, pb := city.Point{X: out, Y: r.Min.Y + inset}, city.Point{X: out, Y: r.Max.Y - inset}
	fillScreen(screen, []city.Point{raised(cam, pa, 0), raised(cam, pb, 0), raised(cam, pb, under), raised(cam, pa, under)}, colorPier)
	edges := sides(cam, r)
	for _, e := range edges {
		fillScreen(screen, []city.Point{raised(cam, e[0], top), raised(cam, e[1], top), raised(cam, e[1], under), raised(cam, e[0], under)}, colorGirder)
	}
	if g.kits != nil {
		g.kitGroundLifted(screen, cam, deckPiece, 0, cell.Center(), top, nil)
	} else {
		c := [4]city.Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, r.Max, {X: r.Min.X, Y: r.Max.Y}}
		fillScreen(screen, []city.Point{raised(cam, c[0], top), raised(cam, c[1], top), raised(cam, c[2], top), raised(cam, c[3], top)}, colorStreet)
	}
	for _, e := range edges {
		railing(screen, cam, e[0], e[1], top, top)
	}
}
