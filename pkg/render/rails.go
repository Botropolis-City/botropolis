package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// The freight loop is the ledger: track drawn on the ground the way the
// wires are drawn in the air, and the kit's trains on it, one per model,
// as long as that model's spend.
const (
	railGauge   = city.Tile * 0.22
	sleeperStep = city.Tile * 0.5
	trainSpeed  = 36.0 // world units per second
	wagonGap    = city.Tile * 0.72
	trainGap    = city.Tile * 2.5
)

var (
	colorRail    = color.NRGBA{0x8c, 0x90, 0x98, 0xff}
	colorSleeper = color.NRGBA{0x4a, 0x42, 0x38, 0xff}
)

// isoRails draws the loop: sleepers across the line, then two rails
// either side of it, projected like everything else on the ground. All
// the sleepers go down in one stroke and all the rail in another, so a
// loop of a hundred cells is two draws, not four hundred.
func (g *Game) isoRails(screen *ebiten.Image, cam *city.Camera, c *city.City) {
	if len(c.Rails) < 2 {
		return
	}
	width := float32(math.Max(1, 1.2*cam.Zoom))
	var sleepers, rails vector.Path
	seg := func(path *vector.Path, a, b city.Point) {
		p, q := cam.WorldToScreen(a), cam.WorldToScreen(b)
		path.MoveTo(float32(p.X), float32(p.Y))
		path.LineTo(float32(q.X), float32(q.Y))
	}
	for i := 1; i < len(c.Rails); i++ {
		a, b := c.Rails[i-1], c.Rails[i]
		dx, dy := b.X-a.X, b.Y-a.Y
		length := math.Hypot(dx, dy)
		if length == 0 {
			continue
		}
		ux, uy := dx/length, dy/length
		nx, ny := -uy*railGauge/2, ux*railGauge/2
		for d := sleeperStep / 2; d < length; d += sleeperStep {
			p := city.Point{X: a.X + ux*d, Y: a.Y + uy*d}
			seg(&sleepers, city.Point{X: p.X + nx*1.6, Y: p.Y + ny*1.6}, city.Point{X: p.X - nx*1.6, Y: p.Y - ny*1.6})
		}
		seg(&rails, city.Point{X: a.X + nx, Y: a.Y + ny}, city.Point{X: b.X + nx, Y: b.Y + ny})
		seg(&rails, city.Point{X: a.X - nx, Y: a.Y - ny}, city.Point{X: b.X - nx, Y: b.Y - ny})
	}
	stroke := func(path *vector.Path, w float32, col color.NRGBA) {
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(col)
		vector.StrokePath(screen, path, &vector.StrokeOptions{Width: w}, op)
	}
	stroke(&sleepers, width*1.6, colorSleeper)
	stroke(&rails, width, colorRail)
}

// carriage is one piece of a train on the loop this frame.
type carriage struct {
	at    city.Point
	model string
	turn  int
	train *city.Train
	head  bool
}

// carriages places every train: heads spaced round the loop, wagons
// trailing each by wagonGap, all moving at trainSpeed.
func (g *Game) carriages(c *city.City, seconds float64) []carriage {
	length := pathLength(c.Rails)
	if length <= 0 || len(c.Trains) == 0 {
		return nil
	}
	var out []carriage
	offset := 0.0
	for _, t := range c.Trains {
		head := math.Mod(seconds*trainSpeed+offset, length)
		for w := 0; w <= t.Wagons; w++ {
			d := math.Mod(head-float64(w)*wagonGap+length*4, length)
			at, dir := pointAlong(c.Rails, d)
			model := kitWagons[t.Hue%len(kitWagons)]
			if w == 0 {
				model = kitLocos[t.Hue%len(kitLocos)]
			}
			out = append(out, carriage{at: at, model: model, turn: carTurn(dir), train: t, head: w == 0})
		}
		offset += float64(t.Wagons+1)*wagonGap + trainGap
	}
	return out
}

func (g *Game) drawCarriage(screen *ebiten.Image, cam *city.Camera, k carriage) {
	r := g.kit(screen, cam, k.model, k.turn, k.at, nil)
	g.noteHit(r, city.Hit{Train: k.train})
}
