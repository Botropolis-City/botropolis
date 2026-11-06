package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// Each group is a line in neutral ink, drawn in the form group_line_form
// says: between its districts, as its neighbourhoods' outlines and the
// links between them, or session to session (item 348). Stations are no
// longer drawn; whose move a site is shows on its buildings and its
// district's sign, and what a site waits on on its buildings' cards.
var (
	colorLineInk    = color.NRGBA{0xe8, 0xe6, 0xdf, 0xff}
	colorLineCasing = color.NRGBA{0x1d, 0x20, 0x28, 0xff}
)

// lineFaded is how much of a line is left while another group has the
// focus.
const lineFaded = 0x40

// groupLines draws the groups' lines in the scene's form. With lit set
// it draws only the links a hovered car keeps lit, at full strength over
// the faded rest, which a call without it draws and leaves them out of
// (item 108).
func (g *Game) groupLines(screen *ebiten.Image, cam *city.Camera, lit bool) {
	c := g.scene.City()
	width := math.Max(2, 3*cam.Zoom)
	dash := 6 * cam.Zoom
	focus := g.scene.FocusedGroup()
	form := g.scene.LineForm()
	for _, s := range g.scene.Shapes() {
		ink, casing := colorLineInk, colorLineCasing
		if focus != "" && s.Group != focus {
			ink.A, casing.A = lineFaded, lineFaded
		}
		var strokes []lineStroke
		for i, l := range s.Links {
			if g.linkLit(c, l) != lit {
				continue
			}
			site := s.LinkSites[i]
			strokes = append(strokes, lineStroke{a: cam.WorldToScreen(l[0]), b: cam.WorldToScreen(l[1]), unlit: site.Unlit, dashed: site.Dashed})
		}
		if !lit {
			for _, o := range s.Outlines {
				for _, e := range o {
					strokes = append(strokes, lineStroke{a: cam.WorldToScreen(e[0]), b: cam.WorldToScreen(e[1])})
				}
			}
		}
		if len(strokes) == 0 {
			continue
		}
		draw := func() { g.strokeLines(screen, strokes, width, dash, ink, casing) }
		if lit {
			draw()
			continue
		}
		// Its strokes are kept while they are drawn from the same
		// (item 212).
		g.keepOverlay(kindLines, g.lineKey(form, strokes, width, dash, ink, casing), draw)
	}
}

// lineStroke is one stroke of a line on screen: a link, unlit while the
// site it leads into waits on another and dashed when it joined on a
// guess, or an edge of a neighbourhood's outline.
type lineStroke struct {
	a, b          city.Point
	unlit, dashed bool
}

// strokeLines draws a line's strokes, cased: an unlit one in the casing
// alone, as an unlit station's centre was.
func (g *Game) strokeLines(screen *ebiten.Image, strokes []lineStroke, width, dash float64, ink, casing color.NRGBA) {
	for _, s := range strokes {
		if s.dashed {
			g.dashed(screen, s.a, s.b, width+2, dash, casing)
			continue
		}
		g.strokeLine(screen, float32(s.a.X), float32(s.a.Y), float32(s.b.X), float32(s.b.Y), float32(width+2), casing)
	}
	for _, s := range strokes {
		fill := ink
		if s.unlit {
			fill = casing
		}
		if s.dashed {
			g.dashed(screen, s.a, s.b, width, dash, fill)
			continue
		}
		g.strokeLine(screen, float32(s.a.X), float32(s.a.Y), float32(s.b.X), float32(s.b.Y), float32(width), fill)
	}
}

// linkLit is whether a link touches a district a hovered car keeps lit,
// so it is drawn over the fade rather than in it (item 108's rule for
// stations).
func (g *Game) linkLit(c *city.City, l [2]city.Point) bool {
	for _, p := range l {
		for _, d := range c.Districts {
			if d.Rect.Contains(p) && g.stationLit(d) {
				return true
			}
		}
	}
	return false
}
