package render

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

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

// drawWindows draws a building's lit windows over the dark: each lit
// pane of its sprite, from the atlas page, where the sprite stands risen
// as far as it has, all in one call.
func (g *Game) drawWindows(screen *ebiten.Image, cam *city.Camera, b *city.Building, lights litWindows, rise float64) {
	atlas := g.kits.pick(cam.Zoom)
	sprite, ok := atlas.Sprite(buildingPiece(b), cam.Heading)
	if !ok || sprite.Page >= len(atlas.pages) {
		return
	}
	scale := cam.Zoom / atlas.Zoom
	foot := g.footOf(cam, b)
	a := float32(lights.alpha)
	vertices := make([]ebiten.Vertex, 0, 4*len(sprite.Panes))
	indices := make([]uint16, 0, 6*len(sprite.Panes))
	for i, q := range sprite.Panes {
		if !lights.all && (i >= len(lights.of) || lights.of[i] >= lights.storeys) {
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
	if len(indices) == 0 {
		return
	}
	screen.DrawTriangles(vertices, indices, atlas.pages[sprite.Page], &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	g.windowDraws++
}

// beginLights starts the night's lights layer for a frame of a size:
// from here every lit building lays its windows in it as it is drawn,
// and every sprite cuts its own shape out of what is already there.
func (g *Game) beginLights(width, height int) {
	if g.lights == nil || g.lights.Bounds().Dx() != width || g.lights.Bounds().Dy() != height {
		if g.lights != nil {
			g.lights.Deallocate()
		}
		g.lights = ebiten.NewImage(width, height)
	}
	g.lights.Clear()
	g.lightsOn, g.windowDraws = true, 0
}

// endLights puts the night's lit windows over the frame, after the wash.
func (g *Game) endLights(screen *ebiten.Image) {
	if !g.lightsOn {
		return
	}
	g.lightsOn = false
	screen.DrawImage(g.lights, nil)
}

// punch cuts a sprite drawn with geo out of the lights laid so far: a
// window behind it does not show through it.
func (g *Game) punch(img *ebiten.Image, geo ebiten.GeoM) {
	if !g.lightsOn {
		return
	}
	op := &ebiten.DrawImageOptions{GeoM: geo, Blend: ebiten.BlendDestinationOut, Filter: ebiten.FilterLinear}
	g.lights.DrawImage(img, op)
}

// lightBuilding lays a building's lit windows, if it has any tonight,
// once it is drawn and before anything in front of it is.
func (g *Game) lightBuilding(cam *city.Camera, b *city.Building) {
	if !g.lightsOn {
		return
	}
	rise := g.scene.Rising(b.Session.ID)
	if lights, ok := g.windowLights(cam, b, rise, g.scene.Lights(b.Session.ID)); ok {
		g.drawWindows(g.lights, cam, b, lights, rise)
	}
}
