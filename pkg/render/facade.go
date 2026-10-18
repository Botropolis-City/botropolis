package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// A stopped building's windows are its own: the atlas carries every
// pane each building sprite shows (item 86), so the glass goes dark and
// the planks are nailed where the windows are.
var (
	colorDarkGlass = color.NRGBA{0x1f, 0x26, 0x33, 0xd8}
	colorPlankWood = color.NRGBA{0x9a, 0x6e, 0x45, 0xff}
	colorPlankEdge = color.NRGBA{0x5e, 0x3f, 0x26, 0xff}
	colorNail      = color.NRGBA{0x2b, 0x2b, 0x2b, 0xff}
)

// plankRows are where down a window's frame its three planks run.
var plankRows = [3]float64{0.17, 0.5, 0.83}

// frame is how far a window's frame and sill reach past its glass, as
// shares of the pane: to each side, above, and below, where the sill is.
const (
	frameSide  = 0.3
	frameAbove = 0.3
	frameSill  = 0.4
)

// planksFor is the planks across one window: three boards along its
// face, covering its frame and sill rather than only the glass, each a
// little skewed and running just past the frame, as the corners of each
// board, top left first. seed keeps a window's boards the same from
// frame to frame.
func planksFor(pane [4]city.Point, seed uint32) [][4]city.Point {
	tl, tr, br, bl := pane[0], pane[1], pane[2], pane[3]
	height := ((bl.Y - tl.Y) + (br.Y - tr.Y)) / 2
	if height <= 0 || tr.X <= tl.X {
		return nil
	}
	run := tr.Sub(tl)
	// The frame reaches a share of the pane each side, but never further
	// than the pane is tall: a ribbon window's frame is a frame, not a
	// share of the whole face (item 109).
	side := frameSide
	if length := math.Hypot(run.X, run.Y); length > 0 {
		side = min(frameSide, height/length)
	}
	above, below := city.Point{Y: -frameAbove * height}, city.Point{Y: frameSill * height}
	tl, bl = tl.Sub(run.Scale(side)).Add(above), bl.Sub(run.Scale(side)).Add(below)
	tr, br = tr.Add(run.Scale(side)).Add(above), br.Add(run.Scale(side)).Add(below)
	framed := height * (1 + frameAbove + frameSill)
	thick := framed * 0.12
	out := make([][4]city.Point, 0, len(plankRows))
	for i, row := range plankRows {
		tilt := city.Point{Y: (float64((seed>>(4*i))%5) - 2) * 0.02 * (tr.X - tl.X)}
		left := lerp(tl, bl, row).Add(tilt)
		right := lerp(tr, br, row).Sub(tilt)
		reach := right.Sub(left).Scale(min(0.06, 0.2*height/max(1e-9, math.Hypot(right.X-left.X, right.Y-left.Y))))
		left, right = left.Sub(reach), right.Add(reach)
		up := city.Point{Y: -thick}
		out = append(out, [4]city.Point{left.Add(up), right.Add(up), right.Sub(up), left.Sub(up)})
	}
	return out
}

func lerp(a, b city.Point, t float64) city.Point {
	return city.Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

// facadePanes is every pane of a piece as drawn with its foot on a
// screen point this frame, in screen pixels.
func (g *Game) facadePanes(cam *city.Camera, name string, foot city.Point) [][4]city.Point {
	if g.kits == nil {
		return nil
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return nil
	}
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok {
		return nil
	}
	scale := cam.Zoom / atlas.Zoom
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	out := make([][4]city.Point, 0, len(sprite.Panes))
	for _, p := range sprite.Panes {
		var q [4]city.Point
		for i, c := range p {
			q[i] = city.Point{X: origin.X + float64(c.X)*scale, Y: origin.Y + float64(c.Y)*scale}
		}
		out = append(out, q)
	}
	return out
}

// shutter darkens a stopped building's glass and, once it is shuttered,
// boards each pane.
func (g *Game) shutter(screen *ebiten.Image, cam *city.Camera, b *city.Building, name string, boarded bool) {
	panes := g.facadePanes(cam, name, g.footOf(cam, b))
	seed := hashID(b.Session.ID)
	for i, pane := range panes {
		fillQuad(screen, pane, colorDarkGlass)
		if !boarded {
			continue
		}
		for _, plank := range planksFor(pane, seed+uint32(i)*2654435761) {
			fillQuad(screen, plank, colorPlankWood)
			strokeQuad(screen, plank, float32(math.Max(1, 0.6*cam.Zoom)), colorPlankEdge)
			radius := float32(math.Max(0.8, 0.9*cam.Zoom))
			for _, end := range []city.Point{lerp(plank[0], plank[3], 0.5), lerp(plank[1], plank[2], 0.5)} {
				nail := lerp(end, lerp(plank[0], plank[2], 0.5), 0.12)
				vector.FillCircle(screen, float32(nail.X), float32(nail.Y), radius, colorNail, true)
			}
		}
	}
}

func quadPath(q [4]city.Point) *vector.Path {
	var p vector.Path
	p.MoveTo(float32(q[0].X), float32(q[0].Y))
	for _, c := range q[1:] {
		p.LineTo(float32(c.X), float32(c.Y))
	}
	p.Close()
	return &p
}

func fillQuad(screen *ebiten.Image, q [4]city.Point, c color.NRGBA) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(screen, quadPath(q), nil, op)
}

func strokeQuad(screen *ebiten.Image, q [4]city.Point, width float32, c color.NRGBA) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.StrokePath(screen, quadPath(q), &vector.StrokeOptions{Width: width}, op)
}
