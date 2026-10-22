package render

import (
	"cmp"
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// A building's windows at night are its own glass at its day brightness
// over the dark (Aria, 2026-10-19, item 166): the sprite's own pixels
// inside its panes, the panes its boards are nailed across, drawn from
// the atlas page again over the night wash in one call, every pane once
// the building stands and those of its finished storeys while it rises.
// Nothing is cut or kept for it: the page is already up.

// litWindows is how a building's windows are lit tonight: how many of its
// storeys are lit, whether that is all of them, how far on its lights
// are, and which storey each of its panes is on.
type litWindows struct {
	storeys int
	all     bool
	alpha   float64
	of      []int
}

// paneHeights is how high each of a kind sprite's panes stands above the
// ground of the face it is on, bottom and top, in the sprite's pixels.
// A face slopes, so a storey is a sloped band, not a row of the sprite:
// the ground under a pane is the footprint's lower edge where the pane
// is, which the scaffold already projects (workFrame).
func paneHeights(s assets.KitSprite, zoom float64, heading int) [][2]float64 {
	cam := &city.Camera{Zoom: zoom, Projection: city.Isometric, Heading: heading}
	rect := city.RectAt(0, 0, city.BuildingSize, city.BuildingSize)
	c := rect.Center()
	foot := cam.WorldToScreen(c)
	c.X += s.Body.X * city.CellSize
	hw, hd := s.Body.W*city.CellSize/2, s.Body.D*city.CellSize/2
	var lot [4]city.Point
	for i, p := range []city.Point{{X: c.X - hw, Y: c.Y - hd}, {X: c.X + hw, Y: c.Y - hd}, {X: c.X + hw, Y: c.Y + hd}, {X: c.X - hw, Y: c.Y + hd}} {
		q := cam.WorldToScreen(p)
		lot[i] = city.Point{X: q.X - foot.X + float64(s.Anchor.X), Y: q.Y - foot.Y + float64(s.Anchor.Y)}
	}
	ground := func(x float64) float64 {
		low := math.Inf(-1)
		for i := range lot {
			a, b := lot[i], lot[(i+1)%4]
			if a.X > b.X {
				a, b = b, a
			}
			if x < a.X || x > b.X || b.X == a.X {
				continue
			}
			low = math.Max(low, a.Y+(b.Y-a.Y)*(x-a.X)/(b.X-a.X))
		}
		if math.IsInf(low, -1) {
			return float64(s.Anchor.Y)
		}
		return low
	}
	out := make([][2]float64, len(s.Panes))
	for i, q := range s.Panes {
		x := float64(q[0].X+q[1].X) / 2
		bottom := float64(q[2].Y+q[3].Y) / 2
		top := float64(q[0].Y+q[1].Y) / 2
		out[i] = [2]float64{ground(x) - bottom, ground(x) - top}
	}
	return out
}

// paneStoreys is which storey each of a kind sprite's panes is on,
// counting from 0 at the ground, by the middle of its height.
func paneStoreys(s assets.KitSprite, zoom float64, heading int) []int {
	storey := storeyHeight(zoom)
	plinth := kindPlinth / kindStorey * storey
	out := make([]int, len(s.Panes))
	for i, h := range paneHeights(s, zoom, heading) {
		out[i] = min(state.MaxStoreys-1, max(0, int(((h[0]+h[1])/2-plinth)/storey)))
	}
	return out
}

// prepareKinds measures what a page's kinds need once its pixels are
// gone: each one's span on every row, for its planks, and the storey of
// each of its panes, for its lights.
func (a *kitAtlas) prepareKinds(index int, page *image.NRGBA) {
	if a.walls == nil {
		a.walls = map[string][4][]span{}
		a.storeys = map[string][4][]int{}
	}
	for _, column := range kinds {
		for _, name := range column {
			walls, storeys := a.walls[name], a.storeys[name]
			for i, heading := range assets.Headings {
				if s, ok := a.Sprite(name, heading); ok && s.Page == index {
					walls[i] = wallSpans(page, s)
					storeys[i] = paneStoreys(s, a.Zoom, heading)
				}
			}
			a.walls[name], a.storeys[name] = walls, storeys
		}
	}
}

// windowLights is how a building's windows are lit tonight, given how
// far it has risen and how far its lights are on: those of each storey
// it has finished (item 155), and none of a building that is not lived
// in, is behind its blinds or boarded up, is ruins, or is on its way.
func (g *Game) windowLights(cam *city.Camera, b *city.Building, rise, on float64) (litWindows, bool) {
	if !b.Lit || b.Blinds || b.Packed || b.Vacant || b.Session.ConversationGone || on <= 0 || rise <= 0 || g.kits == nil {
		return litWindows{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return litWindows{}, false
	}
	name := buildingPiece(b)
	of := atlas.storeys[name][headingIndex(cam.Heading)]
	if len(of) == 0 {
		return litWindows{}, false
	}
	storeys, storey, plinth := g.storeyFrame(cam, b, g.kitSize(cam, name))
	built := rise * (plinth + float64(storeys)*storey)
	n := len(windowRows(storeys, storey, plinth, built))
	if n == 0 {
		return litWindows{}, false
	}
	return litWindows{storeys: n, all: n >= storeys, alpha: min(1, on), of: of}, true
}

// headingIndex is a heading's place in the atlas's order.
func headingIndex(heading int) int {
	return ((heading/90)%4 + 4) % 4
}

// windowTriangles is a building's lit panes as triangles over the atlas
// page they come from, where the sprite stands risen as far as it has.
func (g *Game) windowTriangles(cam *city.Camera, b *city.Building, lights litWindows, rise float64) ([]ebiten.Vertex, []uint16, *ebiten.Image) {
	atlas := g.kits.pick(cam.Zoom)
	sprite, ok := atlas.Sprite(buildingPiece(b), cam.Heading)
	if !ok || sprite.Page >= len(atlas.pages) {
		return nil, nil, nil
	}
	keep := func(i int) bool {
		return lights.all || (i < len(lights.of) && lights.of[i] < lights.storeys)
	}
	vertices, indices := paneVertices(sprite, g.footOf(cam, b), cam.Zoom/atlas.Zoom, rise, lights.alpha, keep)
	if len(indices) == 0 {
		return nil, nil, nil
	}
	return vertices, indices, atlas.pages[sprite.Page]
}

// paneVertices is a sprite's panes that keep says to light, as triangles
// over its atlas page, where the sprite stands on foot at scale, risen
// as far as rise, at a strength of alpha.
func paneVertices(sprite assets.KitSprite, foot city.Point, scale, rise, alpha float64, keep func(int) bool) ([]ebiten.Vertex, []uint16) {
	a := float32(alpha)
	vertices := make([]ebiten.Vertex, 0, 4*len(sprite.Panes))
	indices := make([]uint16, 0, 6*len(sprite.Panes))
	for i, q := range sprite.Panes {
		if !keep(i) {
			continue
		}
		base := uint16(len(vertices))
		for _, c := range q {
			vertices = append(vertices, ebiten.Vertex{
				DstX:   float32(foot.X + float64(c.X-sprite.Anchor.X)*scale),
				DstY:   float32(foot.Y + float64(c.Y-sprite.Anchor.Y)*scale*min(1, rise)),
				SrcX:   float32(sprite.Rect.Min.X + c.X),
				SrcY:   float32(sprite.Rect.Min.Y + c.Y),
				ColorR: a, ColorG: a, ColorB: a, ColorA: a,
			})
		}
		indices = append(indices, base, base+1, base+2, base, base+2, base+3)
	}
	return vertices, indices
}

var windowOptions = &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear}

// drawWindows draws a building's lit windows straight onto an image, for
// a building with nothing in front of it, in one call.
func (g *Game) drawWindows(screen *ebiten.Image, cam *city.Camera, b *city.Building, lights litWindows, rise float64) {
	if vertices, indices, page := g.windowTriangles(cam, b, lights, rise); page != nil {
		g.recorder().triangles16(screen, vertices, indices, page, windowOptions)
		g.windowDraws++
	}
}

// lightOp is one step of laying the night's lights, kept in the order
// the scene was drawn: a building's windows laid, or a sprite in front
// of them cut out.
type lightOp struct {
	vertices []ebiten.Vertex
	indices  []uint16
	src      *ebiten.Image
	geo      ebiten.GeoM
	punch    bool
	// page is the texture a punch's sprite is cut from, so a page's
	// punches are laid together, and rank its place among the pages
	// while cuts are put in order (item 199).
	page *ebiten.Image
	rank int
	// warm is whether the panes are tinted toward lit glass: a
	// landmark's are (item 185).
	warm bool
	// cut is what a painted shape in front cuts the lights with, from
	// and to its range in the cut buffers, alpha its strength, words and
	// face a painted word's (item 195).
	cut cutKind
	// box is the screen box the op covers, so a window laid after a cut
	// waits for it only when they meet.
	box      image.Rectangle
	from, to int
	alpha    float32
	words    string
	face     text.Face
}

// beginLights starts the night's lights for a frame of a size: from here
// every lit building's windows, and every sprite drawn in front of any,
// are noted in depth order, to be laid once the scene is drawn.
func (g *Game) beginLights(width, height int) {
	if g.lights == nil || g.lights.Bounds().Dx() != width || g.lights.Bounds().Dy() != height {
		if g.lights != nil {
			g.lights.Deallocate()
		}
		g.lights = sizedImage(nil, image.Pt(width, height))
	}
	g.lightOps = g.lightOps[:0]
	g.litBounds = image.Rectangle{}
	g.litRects = g.litRects[:0]
	g.cutVertices, g.cutIndices, g.cutPoints = g.cutVertices[:0], g.cutIndices[:0], g.cutPoints[:0]
	g.lightsOn, g.windowDraws, g.landmarkDraws, g.punches, g.cuts = true, 0, 0, 0, 0
	g.meshCut = -1
	g.paint.Cut = lightCutter{g}
}

// endLights lays the night's lights in the order noted, each sprite and
// painted shape cutting out what was laid behind it, and puts them over
// the frame, after the wash. Laid here rather than as the scene is
// drawn, the scene's own draws stay batched together; and each cut
// waits to be laid until a window it meets, or the end, so the cuts
// batch together too.
func (g *Game) endLights(screen *ebiten.Image) {
	if !g.lightsOn {
		return
	}
	g.lightsOn = false
	g.paint.Cut = nil
	if len(g.lightOps) == 0 {
		return
	}
	g.lights.Clear()
	g.layLights(g.lights, g.lightOps)
	screen.DrawImage(g.lights, nil)
}

// layLights lays ops onto dst in the order noted, each sprite and
// painted shape cutting out what was laid behind it; dst may be a
// clipped part of a lights image (item 193).
func (g *Game) layLights(dst *ebiten.Image, ops []lightOp) {
	pending := g.pendingCuts[:0]
	for _, op := range ops {
		if op.punch || op.cut != cutNone {
			pending = append(pending, op)
			continue
		}
		due, kept := g.dueCuts[:0], pending[:0]
		for _, c := range pending {
			if meets(c.box, g.litRects[op.from:op.to]) {
				due = append(due, c)
			} else {
				kept = append(kept, c)
			}
		}
		g.layCuts(dst, due)
		g.dueCuts, pending = due[:0], kept
		if op.warm {
			colorm.DrawTriangles(dst, op.vertices, op.indices, op.src, warmGlass, warmOptions)
			continue
		}
		dst.DrawTriangles(op.vertices, op.indices, op.src, windowOptions)
	}
	g.layCuts(dst, pending)
	g.pendingCuts = pending[:0]
}

// meets is whether a cut covers any of a window's panes. A cut laid
// after a window it does not cover leaves it as it was, so a window
// waits only for the cuts noted before it that it meets, and the rest
// batch with the cuts after (item 195). A pane's light spreads
// litMargin past it, and a cut over only that is met too: otherwise it
// waited for whichever window came next that it did meet, so the pixel
// it shared with this one came out lit or cut by what else was laid,
// and the standing city lays fewer windows than the whole frame does
// (item 193).
func meets(box image.Rectangle, panes []image.Rectangle) bool {
	for _, p := range panes {
		if box.Overlaps(p.Inset(-litMargin)) {
			return true
		}
	}
	return false
}

// layCuts lays the cuts noted between two windows' lights. Each only
// takes light away, so their order among themselves does not change the
// result, and they are laid in orderCuts' order, so that each kind
// batches into as few draws as it can (item 195).
func (g *Game) layCuts(dst *ebiten.Image, ops []lightOp) {
	orderCuts(ops)
	for _, op := range ops {
		g.layCut(dst, op)
	}
}

// orderCuts puts cuts in the order they are laid: a kind at a time, by
// cutOrder, and among punches a page at a time, the pages in the order
// first noted, and a page's largest first; otherwise in the order
// noted. Every stamp and
// painted mesh is cut from the painter's one sheet, so each page and
// each kind is a draw, and the same draws every run: sorted by where
// each sprite's image was allocated, the pages interleaved differently
// by run (item 199).
func orderCuts(ops []lightOp) {
	orderCutsBy(ops, func(op *lightOp) *lightOp { return op })
}

// orderCutsBy is orderCuts over anything holding a light op.
func orderCutsBy[T any](cuts []T, op func(*T) *lightOp) {
	seen := make([]*ebiten.Image, 0, 8)
	for i := range cuts {
		o := op(&cuts[i])
		o.rank = 0
		if !o.punch {
			continue
		}
		o.rank = slices.Index(seen, o.page)
		if o.rank < 0 {
			o.rank = len(seen)
			seen = append(seen, o.page)
		}
	}
	slices.SortStableFunc(cuts, func(a, b T) int {
		x, y := op(&a), op(&b)
		return cmp.Or(cmp.Compare(cutOrder(*x), cutOrder(*y)), cmp.Compare(x.rank, y.rank), cmp.Compare(punchSize(*y), punchSize(*x)))
	})
}

// punchSize is how much a punch's geo scales its sprite's area, which
// picks the mipmap Ebitengine draws it from, each a texture of its own:
// a page's punches are laid largest first, so each mipmap is one draw.
// Any other cut is drawn without mipmaps, and has none.
func punchSize(op lightOp) float64 {
	if !op.punch {
		return 0
	}
	g := op.geo
	return math.Abs(g.Element(0, 0)*g.Element(1, 1) - g.Element(0, 1)*g.Element(1, 0))
}

// punch notes a sprite drawn with geo, cut from page, to be cut out of
// any lights laid behind it, when it covers any: a window behind it does
// not show through it.
func (g *Game) punch(img, page *ebiten.Image, geo ebiten.GeoM) {
	if !g.lightsOn || g.baking || g.litBounds.Empty() {
		return
	}
	b := img.Bounds()
	box := image.Rectangle{}
	for i, c := range [][2]float64{{0, 0}, {float64(b.Dx()), 0}, {0, float64(b.Dy())}, {float64(b.Dx()), float64(b.Dy())}} {
		x, y := geo.Apply(c[0], c[1])
		p := image.Rect(int(math.Floor(x)), int(math.Floor(y)), int(math.Ceil(x))+1, int(math.Ceil(y))+1)
		if i == 0 {
			box = p
		} else {
			box = box.Union(p)
		}
	}
	if !box.Overlaps(g.litBounds) {
		return
	}
	g.lightOps = append(g.lightOps, lightOp{src: img, page: page, geo: geo, punch: true, box: box})
	g.punches++
}

// lightBuilding notes a building's lit windows, if it has any tonight,
// once it is drawn and before anything in front of it is.
func (g *Game) lightBuilding(cam *city.Camera, b *city.Building) {
	if !g.lightsOn {
		return
	}
	rise := g.scene.Rising(b.Session.ID)
	lights, ok := g.windowLights(cam, b, rise, g.scene.Lights(b.Session.ID))
	if !ok {
		return
	}
	g.noteLights(g.windowTriangles(cam, b, lights, rise))
}

// noteLights notes lit panes to be laid in depth order, in one draw, and
// widens what any sprite drawn after must cut out of.
func (g *Game) noteLights(vertices []ebiten.Vertex, indices []uint16, page *ebiten.Image) {
	g.noteLightsTinted(vertices, indices, page, false)
}

// noteLightsTinted notes lit panes as noteLights does, tinted warm or not.
func (g *Game) noteLightsTinted(vertices []ebiten.Vertex, indices []uint16, page *ebiten.Image, warm bool) {
	if page == nil {
		return
	}
	step := 3
	if len(indices)%6 == 0 {
		step = 6
	}
	first := len(g.litRects)
	for i := 0; i+step <= len(indices); i += step {
		pane := image.Rectangle{}
		for j, k := range indices[i : i+step] {
			v := vertices[k]
			p := pixelAround(float64(v.DstX), float64(v.DstY))
			if j == 0 {
				pane = p
			} else {
				pane = pane.Union(p)
			}
		}
		g.litBounds = g.litBounds.Union(pane)
		g.litRects = append(g.litRects, pane)
	}
	g.meshCut = -1
	g.lightOps = append(g.lightOps, lightOp{vertices: vertices, indices: indices, src: page, warm: warm, from: first, to: len(g.litRects)})
	g.windowDraws++
}
