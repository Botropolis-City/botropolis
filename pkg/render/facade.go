package render

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/assets"
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

// ribbonRatio is how many times longer than it is tall a pane must run
// to be a ribbon window: any wider than tall, since every kind's own
// windows stand taller than they are wide, and a ribbon hidden in part
// by a crane is a short piece of ribbon still.
const ribbonRatio = 1.0

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
	// A ribbon runs to the edge of its face, so its planks stop at its
	// glass: a frame past either end would hang off the building (item
	// 166).
	side, ribbon := frameSide, false
	if length := math.Hypot(run.X, run.Y); length > 0 {
		side = min(frameSide, height/length)
		ribbon = length >= ribbonRatio*height
	}
	if ribbon {
		side = 0
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
		if ribbon {
			reach = city.Point{}
		}
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

// span is the run of a sprite's row that is the building: its first
// opaque pixel and the one after its last.
type span struct{ lo, hi int }

// wallSpans is each row of a sprite's span, so a plank can be held to
// the building's own pixels (item 166).
func wallSpans(page *image.NRGBA, s assets.KitSprite) []span {
	rows := make([]span, s.Rect.Dy())
	for y := range rows {
		lo, hi := s.Rect.Dx(), 0
		for x := range s.Rect.Dx() {
			if page.NRGBAAt(s.Rect.Min.X+x, s.Rect.Min.Y+y).A > 0 {
				lo, hi = min(lo, x), x+1
			}
		}
		rows[y] = span{lo, hi}
	}
	return rows
}

// boardsOf is the planks over each pane of a sprite drawn with its foot
// on a screen point at scale, nailed to its walls' rows. They are laid
// and nailed in the sprite's own pixels and only then put on screen, so
// a camera that pans tips no tie: not a square pane between ribbon and
// window, nor a corner on a row's edge between two rows (item 237).
func boardsOf(sprite assets.KitSprite, rows []span, foot city.Point, scale float64, seed uint32) [][][4]city.Point {
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	out := make([][][4]city.Point, len(sprite.Panes))
	for i, p := range sprite.Panes {
		var pane [4]city.Point
		for j, c := range p {
			pane[j] = city.Point{X: float64(c.X), Y: float64(c.Y)}
		}
		out[i] = planksFor(pane, seed+uint32(i)*2654435761)
		nail(out[i], rows)
		for p := range out[i] {
			for j, c := range out[i][p] {
				out[i][p][j] = origin.Add(c.Scale(scale))
			}
		}
	}
	return out
}

// nail holds each corner of each plank to the building's own span on
// its row, so a plank stops at the wall's edge rather than hanging off
// it. The planks are in the sprite's own pixels.
func nail(planks [][4]city.Point, rows []span) {
	if len(rows) == 0 {
		return
	}
	for i := range planks {
		for j, c := range planks[i] {
			row := rows[min(len(rows)-1, max(0, int(c.Y)))]
			if row.hi <= row.lo {
				continue
			}
			planks[i][j].X = min(float64(row.hi), max(float64(row.lo), c.X))
		}
	}
}

// shutter darkens a stopped building's glass and, once it is shuttered,
// boards each pane.
func (g *Game) shutter(screen *ebiten.Image, cam *city.Camera, b *city.Building, name string, boarded bool) {
	foot := g.footOf(cam, b)
	panes := g.facadePanes(cam, name, foot)
	var boards [][][4]city.Point
	if atlas := g.kitAtlas(cam); atlas != nil {
		if sprite, ok := atlas.Sprite(name, cam.Heading); ok {
			boards = boardsOf(sprite, atlas.walls[name][headingIndex(cam.Heading)], foot, cam.Zoom/atlas.Zoom, hashID(b.Session.ID))
		}
	}
	if !boarded {
		// Panes never overlap, so one mesh draws them as one draw each
		// would (item 161).
		for _, pane := range panes {
			g.paint.AddFill(screen, pane[:], colorDarkGlass)
		}
		g.paint.Flush(screen)
		return
	}
	// A plank covers the nails of the planks under it, and nails are
	// stamps, not mesh, so each plank goes down as one mesh with its edge,
	// and the glass under it with the first, before its own nails: the
	// order they were drawn in one at a time. Stamping every nail after
	// one mesh for the building showed the covered ones (item 161).
	edge := math.Max(1, 0.6*cam.Zoom)
	radius := float32(math.Max(0.8, 0.9*cam.Zoom))
	for i, pane := range panes {
		g.paint.AddFill(screen, pane[:], colorDarkGlass)
		for _, plank := range boards[i] {
			g.paint.AddFill(screen, plank[:], colorPlankWood)
			g.paint.AddStroke(plank[:], edge, true, colorPlankEdge)
			g.paint.Flush(screen)
			for _, n := range plankNails(plank) {
				g.fillCircle(screen, float32(n.X), float32(n.Y), radius, colorNail)
			}
		}
		g.paint.Flush(screen)
	}
}

// shutterEach is shutter drawn a draw at a time, as it was before item
// 161, for the check that the one mesh draws the same.
func (g *Game) shutterEach(screen *ebiten.Image, cam *city.Camera, b *city.Building, name string) {
	foot := g.footOf(cam, b)
	panes := g.facadePanes(cam, name, foot)
	var boards [][][4]city.Point
	if atlas := g.kitAtlas(cam); atlas != nil {
		if sprite, ok := atlas.Sprite(name, cam.Heading); ok {
			boards = boardsOf(sprite, atlas.walls[name][headingIndex(cam.Heading)], foot, cam.Zoom/atlas.Zoom, hashID(b.Session.ID))
		}
	}
	for i, pane := range panes {
		g.fillQuad(screen, pane, colorDarkGlass)
		for _, plank := range boards[i] {
			g.fillQuad(screen, plank, colorPlankWood)
			g.strokeQuad(screen, plank, float32(math.Max(1, 0.6*cam.Zoom)), colorPlankEdge)
			radius := float32(math.Max(0.8, 0.9*cam.Zoom))
			for _, n := range plankNails(plank) {
				g.fillCircle(screen, float32(n.X), float32(n.Y), radius, colorNail)
			}
		}
	}
}

// plankNails are where a plank is nailed: near each end, a little in.
func plankNails(plank [4]city.Point) [2]city.Point {
	var out [2]city.Point
	for i, end := range []city.Point{lerp(plank[0], plank[3], 0.5), lerp(plank[1], plank[2], 0.5)} {
		out[i] = lerp(end, lerp(plank[0], plank[2], 0.5), 0.12)
	}
	return out
}

func (g *Game) fillQuad(screen *ebiten.Image, q [4]city.Point, c color.NRGBA) {
	g.paint.Fill(screen, q[:], c)
}

func (g *Game) strokeQuad(screen *ebiten.Image, q [4]city.Point, width float32, c color.NRGBA) {
	g.paint.Stroke(screen, q[:], float64(width), true, c)
}
