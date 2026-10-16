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

// plankRows are where across a pane's height the two planks run.
var plankRows = [2]float64{0.3, 0.68}

// planksFor is the planks across one pane: two boards along its face,
// each a little skewed and running past both sides, as the corners of
// each board, top left first. seed keeps a pane's boards the same from
// frame to frame.
func planksFor(pane [4]city.Point, seed uint32) [][4]city.Point {
	tl, tr, br, bl := pane[0], pane[1], pane[2], pane[3]
	height := ((bl.Y - tl.Y) + (br.Y - tr.Y)) / 2
	if height <= 0 || tr.X <= tl.X {
		return nil
	}
	thick := height * 0.11
	out := make([][4]city.Point, 0, len(plankRows))
	for i, row := range plankRows {
		tilt := city.Point{Y: (float64((seed>>(4*i))%5) - 2) * 0.02 * (tr.X - tl.X)}
		left := lerp(tl, bl, row).Add(tilt)
		right := lerp(tr, br, row).Sub(tilt)
		run := right.Sub(left)
		left, right = left.Sub(run.Scale(0.18)), right.Add(run.Scale(0.18))
		up := city.Point{Y: -thick}
		out = append(out, [4]city.Point{left.Add(up), right.Add(up), right.Sub(up), left.Sub(up)})
	}
	return out
}

func lerp(a, b city.Point, t float64) city.Point {
	return city.Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

// facadePanes is every pane of a piece as drawn at its foot this frame,
// in screen pixels.
func (g *Game) facadePanes(cam *city.Camera, name string, at city.Point) [][4]city.Point {
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
	foot := cam.WorldToScreen(at)
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
	panes := g.facadePanes(cam, name, b.Rect.Center())
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
