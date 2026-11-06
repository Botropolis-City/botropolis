package render

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// hoodMarks is what a neighbourhood paints on the ground for a look:
// the block's ground paved as one, a kerb round its outline, and a gate
// at its corner, lettered or bare (item 348, slice 2).
type hoodMarks struct {
	pave  []city.Rect
	kerb  [][2]city.Point
	gate  bool
	at    city.Point
	label string
}

func neighbourhoodMarks(n city.Neighbourhood, look city.NeighbourhoodLook) hoodMarks {
	var m hoodMarks
	if look.Ground {
		m.pave = n.Ground
	}
	if look.Kerb {
		m.kerb = n.Edges()
	}
	if look.Gate {
		m.gate, m.at = true, n.Gate
		if look.Name && n.Group != nil {
			m.label = n.Group.Short
		}
	}
	return m
}

// gatePost is how tall a neighbourhood's gate stands, in screen pixels
// at zoom 1.
const gatePost = 22.0

// neighbourhoods paints every district's neighbourhoods into the ground
// layer, over the district's floor and under its buildings. Only the
// colours the ground already has: the plaza's paving and the kerb.
func (g *Game) neighbourhoods(screen *ebiten.Image, c *city.City, cam *city.Camera) {
	look := g.scene.NeighbourhoodLook()
	if look == (city.NeighbourhoodLook{}) {
		return
	}
	for _, d := range c.Districts {
		for _, n := range c.Neighbourhoods(d) {
			m := neighbourhoodMarks(n, look)
			for _, r := range m.pave {
				g.poly(screen, cam, r, colorKitConcrete)
			}
			if len(m.kerb) > 0 {
				pairs := g.linePairs[:0]
				for _, e := range m.kerb {
					pairs = append(pairs, cam.WorldToScreen(e[0]), cam.WorldToScreen(e[1]))
				}
				g.paint.Lines(screen, pairs, math.Max(1.5, 2.5*cam.Zoom), colorKitKerb)
				g.linePairs = pairs[:0]
			}
			if m.gate {
				g.gate(screen, cam, m)
			}
		}
	}
}

// gate stands a post at a neighbourhood's corner with a small board on
// it, lettered with the group's short name when the look says so.
func (g *Game) gate(screen *ebiten.Image, cam *city.Camera, m hoodMarks) {
	foot := cam.WorldToScreen(m.at)
	top := city.Point{X: foot.X, Y: foot.Y - gatePost*cam.Zoom}
	stroke := float32(math.Max(1.5, 2*cam.Zoom))
	g.strokeLine(screen, float32(foot.X), float32(foot.Y), float32(top.X), float32(top.Y), stroke, colorKitKerb)
	w, h := 8*cam.Zoom, 6*cam.Zoom
	if m.label != "" {
		tw, th := g.measure(m.label)
		w, h = tw+8, th+4
	}
	board := city.RectAt(top.X-w/2, top.Y-h, w, h)
	g.paint.Box(screen, float32(board.Min.X), float32(board.Min.Y), float32(board.Width()), float32(board.Height()), colorKitKerb)
	g.paint.BoxStroke(screen, float32(board.Min.X), float32(board.Min.Y), float32(board.Width()), float32(board.Height()), 1, colorKitFloor)
	if m.label != "" {
		g.label(screen, city.Point{X: board.Min.X + 4, Y: board.Min.Y + 2}, m.label, colorKitFloor)
	}
}
