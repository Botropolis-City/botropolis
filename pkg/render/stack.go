package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Bug 46b. The stack passes through the roof, and a low curb round the
// opening is what says so.
//
// The curb goes down in two passes with the stack between them, and
// that ordering is the whole trick. The ring is wider than the stack,
// so it wraps past the horizontal diameter and its ends run behind the
// stack's silhouette — 262° of it is visible at this radius, not 180°.
// Drawing the ring whole and letting the stack cover what it covers
// gets that for nothing; computing the visible arc would be a second
// expression of the same fact, free to disagree with the first.
var (
	colorStackCurb = color.NRGBA{0x54, 0x58, 0x60, 0xff}
	colorStackTop  = color.NRGBA{0x71, 0x76, 0x7e, 0xff}
	colorStackRim  = color.NRGBA{0x3a, 0x3e, 0x45, 0xff}
)

const arcSteps = 28

// arc walks an ellipse from one angle to another. Angles run
// anticlockwise on screen from the ellipse's right-hand end, so
// [0, π] is the near half and [π, 2π] the far half.
func arc(box city.Rect, from, to float64) []city.Point {
	cx, cy := box.Center().X, box.Center().Y
	rx, ry := box.Width()/2, box.Height()/2
	out := make([]city.Point, 0, arcSteps+1)
	for i := 0; i <= arcSteps; i++ {
		t := from + (to-from)*float64(i)/arcSteps
		out = append(out, city.Point{X: cx + rx*math.Cos(t), Y: cy + ry*math.Sin(t)})
	}
	return out
}

func pathThrough(points []city.Point) vector.Path {
	var path vector.Path
	for i, p := range points {
		if i == 0 {
			path.MoveTo(float32(p.X), float32(p.Y))
		} else {
			path.LineTo(float32(p.X), float32(p.Y))
		}
	}
	return path
}

func fillPoints(screen *ebiten.Image, points []city.Point, c color.NRGBA) {
	path := pathThrough(points)
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(screen, &path, nil, op)
}

func strokePoints(screen *ebiten.Image, points []city.Point, width float32, c color.NRGBA) {
	path := pathThrough(points)
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: width}, op)
}

// curbBehind draws the whole ring: the cylinder's silhouette, bounded
// above by the top ellipse's far arc and below by the foot ellipse's
// near arc. The stack goes over it next and takes out the 98° that
// really is hidden.
func (g *Game) curbBehind(screen *ebiten.Image, f ui.StackFittings, hair float32) {
	silhouette := append(arc(f.OuterTop, math.Pi, 2*math.Pi), arc(f.OuterFoot, 0, math.Pi)...)
	fillPoints(screen, silhouette, colorStackCurb)
	strokePoints(screen, arc(f.OuterFoot, 0, math.Pi), hair, colorStackRim)
}

// curbFront draws the near half of the curb's top surface: the annulus
// between its outer and inner rims. This is what hides the sprite's
// straight cut, and its inner edge — the stack's own radius, so there
// is no gap at the wall — is the stack's visible bottom line.
func (g *Game) curbFront(screen *ebiten.Image, f ui.StackFittings) {
	annulus := append(arc(f.OuterTop, math.Pi, 0), arc(f.InnerTop, 0, math.Pi)...)
	fillPoints(screen, annulus, colorStackTop)
}

// stackThrough draws a stack rising through a roof, curb and all: the
// ring behind, the stack, then the ring's near top over the cut.
func (g *Game) stackThrough(screen *ebiten.Image, cam *city.Camera, name string, at city.Point, cut float64) city.Rect {
	page, src, origin, scale, rect, ok := g.kitThroughPlace(cam, name, at, cut)
	if !ok {
		return city.Rect{}
	}
	f, laid := ui.LayoutStackFittings(rect, rect.Max.Y)
	hair := float32(math.Max(1, rect.Width()*0.035))
	if laid {
		g.curbBehind(screen, f, hair)
	}
	g.drawSprite(screen, page.SubImage(src).(*ebiten.Image), origin, scale, nil)
	if laid {
		g.curbFront(screen, f)
	}
	return rect
}
