package render

import (
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// The bridge off the island. Item 132: the rail stays on the ground and
// the road goes over it, as in the real world — the road climbs from the
// landing's centre on a ramp of short open spans (item 146), and the
// deck stands on piers from there out to sea, high enough that the
// tallest train piece passes under it. The deck's surface is the roads
// kit's straight piece; its girders, piers, railings and the ramp are
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
	colorPier    = color.NRGBA{0x5a, 0x5e, 0x66, 0xff}
	colorGirder  = color.NRGBA{0x86, 0x89, 0x90, 0xff}
	colorRailing = color.NRGBA{0xd4, 0xd6, 0xda, 0xff}
	// colorRampRoad and colorRampLine are the deck's road piece's top
	// face and lane line, as the atlas draws them, so the ramp reads as
	// the same road climbing.
	colorRampRoad = color.NRGBA{0x76, 0x7d, 0x96, 0xff}
	colorRampLine = color.NRGBA{0xd0, 0xdc, 0xff, 0xff}
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

// bridgePart is a piece of the bridge in the depth sort, running From
// one distance out along the bridge To another: a sloped segment, or a
// span, level from end to end.
type bridgePart struct {
	Rect     city.Rect
	Cell     city.Cell
	From, To float64
	Level    bool
	// Haze is whether the part is a piece of the deck cut for the haze,
	// and Mist how far it is mixed toward the sea's colour (item 219).
	Haze bool
	Mist float64
}

// hazeColour is a colour of the deck mist of the way into the sea's.
func hazeColour(c color.NRGBA, mist float64, night bool) color.NRGBA {
	if mist <= 0 {
		return c
	}
	return mix(c, shorePaletteFor(night).sea, mist)
}

// hazy is whether a part is a piece of the deck cut for the haze.
func (p bridgePart) hazy() bool { return p.Haze }

func (p bridgePart) span() bool { return p.Level }

// bridgeParts is the ramp's segments from the landing outward, then the
// deck cell by cell: a cell the road crosses level is a span, and one it
// climbs or comes down over is cut into half-cell segments, and at every
// change of slope, as the ramp is (item 252).
func bridgeParts(b city.Bridge) []bridgePart {
	var parts []bridgePart
	run := b.RampRun()
	half := city.CellSize / 2
	dir := 1.0
	if b.Far.X < b.Land.X {
		dir = -1
	}
	rows := b.Abutment.Rect()
	segment := func(d0, d1 float64) bridgePart {
		x0, x1 := b.Land.X+dir*d0, b.Land.X+dir*d1
		r := city.Rect{Min: city.Point{X: min(x0, x1), Y: rows.Min.Y}, Max: city.Point{X: max(x0, x1), Y: rows.Max.Y}}
		return bridgePart{Rect: r, Cell: city.CellOf(r.Center()), From: d0, To: d1}
	}
	for d := 0.0; d < run-1e-9; d += half {
		parts = append(parts, segment(d, d+half))
	}
	unit := func(h city.Height) float64 { return float64(h) }
	knots := b.Profile()
	along := func(x float64) float64 { return (x - b.Land.X) * dir }
	for _, cell := range b.Deck {
		r := cell.Rect()
		a0, a1 := min(along(r.Min.X), along(r.Max.X)), max(along(r.Min.X), along(r.Max.X))
		cuts := []float64{a0}
		for i := 1; i < len(knots)-1; i++ {
			k := knots[i]
			if knots[i-1].Height == k.Height && knots[i+1].Height == k.Height {
				continue
			}
			if k.Along > a0+1e-9 && k.Along < a1-1e-9 {
				cuts = append(cuts, k.Along)
			}
		}
		if len(cuts) > 1 || b.LiftAt(a0, unit) != b.LiftAt(a1, unit) {
			cuts = append(cuts, a0+half, a1)
			slices.Sort(cuts)
			cuts = slices.Compact(cuts)
			for i := 1; i < len(cuts); i++ {
				parts = append(parts, segment(cuts[i-1], cuts[i]))
				parts[len(parts)-1].Cell = cell
			}
			continue
		}
		if a1 <= along(b.Haze.X) {
			parts = append(parts, bridgePart{Rect: r, Cell: cell, From: a0, To: a1, Level: true})
			continue
		}
		// Into the haze, the deck is cut into quarters, each mixed
		// toward the sea by how little of it shows at its outer end, so the
		// last is the sea's own colour (item 219).
		quarter := city.CellSize / 4
		for i := range 4 {
			x0 := r.Min.X + float64(i)*quarter
			if dir < 0 {
				x0 = r.Max.X - float64(i+1)*quarter
			}
			q := city.Rect{Min: city.Point{X: x0, Y: r.Min.Y}, Max: city.Point{X: x0 + quarter, Y: r.Max.Y}}
			if min(along(q.Min.X), along(q.Max.X)) >= along(b.Far.X) {
				break
			}
			end := city.Point{X: q.Max.X, Y: q.Center().Y}
			if dir < 0 {
				end.X = q.Min.X
			}
			d0 := min(along(q.Min.X), along(q.Max.X))
			parts = append(parts, bridgePart{Rect: q, Cell: cell, From: d0, To: d0 + quarter, Level: true, Haze: true, Mist: 1 - b.Alpha(end)})
		}
	}
	return parts
}

// liftToWorld is a height on screen as a height in the world, so it
// compares with distances on the ground: the 2:1 view draws a world
// unit upright as IsoScale·√2·cos 30° map units.
func liftToWorld(cam *city.Camera, lift float64) float64 {
	return lift / (cam.Zoom * city.IsoScale * math.Sqrt2 * math.Cos(math.Pi/6))
}

// bridgeItems is the bridge in the depth sort, every part raised.
func bridgeItems(cam *city.Camera, b city.Bridge, draw func(p bridgePart) func()) []drawable {
	if len(b.Deck) == 0 {
		return nil
	}
	parts := bridgeParts(b)
	items := make([]drawable, 0, len(parts))
	for _, p := range parts {
		item := footprintAt(cam, p.Rect, draw(p))
		item.level = levelDeck
		items = append(items, item)
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
	if along, ok := b.Along(at); ok && along > 0 {
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

func (g *Game) fillScreen(screen *ebiten.Image, pts []city.Point, c color.NRGBA) {
	g.paint.Fill(screen, pts, c)
}

func (g *Game) strokeScreen(screen *ebiten.Image, a, b city.Point, width float64, c color.NRGBA) {
	g.strokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(width), c)
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
func (g *Game) railing(screen *ebiten.Image, cam *city.Camera, a, b city.Point, liftA, liftB float64) {
	up := railHeight * cam.Zoom
	g.strokeScreen(screen, raised(cam, a, liftA+up), raised(cam, b, liftB+up), math.Max(1, cam.Zoom), colorRailing)
	for i := 0; i <= 4; i++ {
		f := float64(i) / 4
		p := lerpPoint(a, b, f)
		lift := liftA + (liftB-liftA)*f
		g.strokeScreen(screen, raised(cam, p, lift), raised(cam, p, lift+up), math.Max(1, 0.8*cam.Zoom), colorRailing)
	}
}

// drawRampSegment is one piece of the ramp (item 146): a sloped deck on
// a pier at its high end, with girders under the road and railings
// over it, open underneath like the spans out over the water.
func (g *Game) drawRampSegment(screen *ebiten.Image, cam *city.Camera, b city.Bridge, p bridgePart) {
	r := p.Rect
	height := g.heights(cam)
	thick := deckThickness * cam.Zoom
	dir := 1.0
	if b.Far.X < b.Land.X {
		dir = -1
	}
	low, high := b.Land.X+dir*p.From, b.Land.X+dir*p.To
	liftLow, liftHigh := b.LiftAt(p.From, height), b.LiftAt(p.To, height)
	if liftLow > liftHigh {
		low, high, liftLow, liftHigh = high, low, liftHigh, liftLow
	}
	under := func(lift float64) float64 { return max(0, lift-thick) }
	inset := 0.2 * (r.Max.Y - r.Min.Y)
	pa, pb := city.Point{X: high, Y: r.Min.Y + inset}, city.Point{X: high, Y: r.Max.Y - inset}
	if under(liftHigh) > 0 {
		g.fillScreen(screen, []city.Point{raised(cam, pa, 0), raised(cam, pb, 0), raised(cam, pb, under(liftHigh)), raised(cam, pa, under(liftHigh))}, colorPier)
	}
	edges := sides(cam, r)
	along := func(e [2]city.Point) (lo, hi city.Point) {
		if e[0].X == low {
			return e[0], e[1]
		}
		return e[1], e[0]
	}
	for _, e := range edges {
		lo, hi := along(e)
		g.fillScreen(screen, []city.Point{raised(cam, lo, liftLow), raised(cam, hi, liftHigh), raised(cam, hi, under(liftHigh)), raised(cam, lo, under(liftLow))}, colorGirder)
	}
	f, n := edges[0], edges[1]
	flo, fhi := along(f)
	nlo, nhi := along(n)
	g.fillScreen(screen, []city.Point{raised(cam, flo, liftLow), raised(cam, fhi, liftHigh), raised(cam, nhi, liftHigh), raised(cam, nlo, liftLow)}, colorRampRoad)
	mid := r.Center().Y
	g.strokeScreen(screen, raised(cam, city.Point{X: low, Y: mid}, liftLow), raised(cam, city.Point{X: high, Y: mid}, liftHigh), math.Max(1, cam.Zoom), colorRampLine)
	for _, e := range edges {
		lo, hi := along(e)
		g.railing(screen, cam, lo, hi, liftLow, liftHigh)
	}
}

// drawHazePiece is a quarter of a span in the haze (item 219): its
// girders, road, lane line and railings, and the pier where its cell
// ends, every colour mixed toward the sea's by the piece's mist, drawn
// flat as the ramp is, so the deck melts into the water and is gone.
func (g *Game) drawHazePiece(screen *ebiten.Image, cam *city.Camera, b city.Bridge, p bridgePart, night bool) {
	r := p.Rect
	top := b.LiftAt(p.From, g.heights(cam))
	under := top - deckThickness*cam.Zoom
	paint := func(c color.NRGBA) color.NRGBA { return hazeColour(c, p.Mist, night) }
	out, edge := r.Max.X, p.Cell.Rect().Max.X
	if b.Far.X < b.Land.X {
		out, edge = r.Min.X, p.Cell.Rect().Min.X
	}
	if out == edge {
		inset := 0.2 * (r.Max.Y - r.Min.Y)
		pa, pb := city.Point{X: out, Y: r.Min.Y + inset}, city.Point{X: out, Y: r.Max.Y - inset}
		g.fillScreen(screen, []city.Point{raised(cam, pa, 0), raised(cam, pb, 0), raised(cam, pb, under), raised(cam, pa, under)}, paint(colorPier))
	}
	edges := sides(cam, r)
	for _, e := range edges {
		g.fillScreen(screen, []city.Point{raised(cam, e[0], top), raised(cam, e[1], top), raised(cam, e[1], under), raised(cam, e[0], under)}, paint(colorGirder))
	}
	f, n := edges[0], edges[1]
	g.fillScreen(screen, []city.Point{raised(cam, f[0], top), raised(cam, f[1], top), raised(cam, n[1], top), raised(cam, n[0], top)}, paint(colorRampRoad))
	mid := r.Center().Y
	g.strokeScreen(screen, raised(cam, city.Point{X: r.Min.X, Y: mid}, top), raised(cam, city.Point{X: r.Max.X, Y: mid}, top), math.Max(1, cam.Zoom), paint(colorRampLine))
	up := railHeight * cam.Zoom
	for _, e := range edges {
		g.strokeScreen(screen, raised(cam, e[0], top+up), raised(cam, e[1], top+up), math.Max(1, cam.Zoom), paint(colorRailing))
		if out == edge {
			post := e[0]
			if e[1].X == out {
				post = e[1]
			}
			g.strokeScreen(screen, raised(cam, post, top), raised(cam, post, top+up), math.Max(1, 0.8*cam.Zoom), paint(colorRailing))
		}
	}
}

// drawSpan is one cell of the deck: the pier at its outer edge, the
// girders, the road and its railings, at the height the road stands at
// over it.
func (g *Game) drawSpan(screen *ebiten.Image, cam *city.Camera, b city.Bridge, p bridgePart) {
	cell := p.Cell
	r := cell.Rect()
	top := b.LiftAt(p.From, g.heights(cam))
	under := top - deckThickness*cam.Zoom
	out := r.Max.X
	if b.Far.X < b.Land.X {
		out = r.Min.X
	}
	inset := 0.2 * (r.Max.Y - r.Min.Y)
	pa, pb := city.Point{X: out, Y: r.Min.Y + inset}, city.Point{X: out, Y: r.Max.Y - inset}
	g.fillScreen(screen, []city.Point{raised(cam, pa, 0), raised(cam, pb, 0), raised(cam, pb, under), raised(cam, pa, under)}, colorPier)
	edges := sides(cam, r)
	for _, e := range edges {
		g.fillScreen(screen, []city.Point{raised(cam, e[0], top), raised(cam, e[1], top), raised(cam, e[1], under), raised(cam, e[0], under)}, colorGirder)
	}
	if g.kits != nil {
		g.kitGroundLifted(screen, cam, deckPiece, 0, cell.Center(), top, nil)
	} else {
		c := [4]city.Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, r.Max, {X: r.Min.X, Y: r.Max.Y}}
		g.fillScreen(screen, []city.Point{raised(cam, c[0], top), raised(cam, c[1], top), raised(cam, c[2], top), raised(cam, c[3], top)}, colorStreet)
	}
	for _, e := range edges {
		g.railing(screen, cam, e[0], e[1], top, top)
	}
}
