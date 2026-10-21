package render

import (
	"image/color"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
)

// The freight loop is the ledger: track drawn on the ground the way the
// wires are drawn in the air, and the kit's trains on it, one per model,
// as long as that model's spend.
//
// A train is spaced by its own pieces' lengths, read from the atlas
// (item 134). The constants this replaced were in tiles while the
// pieces are in cells, three tiles each, so every wagon stood two thirds
// over the one behind it and nothing measured could say so.
const (
	sleeperStep = city.Tile * 0.5
	trainSpeed  = 36.0 // world units per second
	coupling    = city.Tile * 0.15
	// trainGap is the clear track between two trains, in wagons.
	trainGap = 1.5
)

var vehicleFeet = sync.OnceValue(func() map[string]assets.Foot {
	feet, _ := assets.Footprints()
	return feet
})

// pieceLength is how long a piece is on the loop, in world units.
func pieceLength(model string) float64 {
	return vehicleFeet()[model].Length * city.CellSize
}

// messageCarriage is how long a message's carriage on a group's line is,
// in cells: the size it had before the train grew, kept because it rides
// over lots, badges and station dots (item 134).
const messageCarriage = 0.6075

// messageShrink scales a piece to messageCarriage, whatever scale the
// pipeline cut it at.
func messageShrink(model string) float64 {
	length := vehicleFeet()[model].Length
	if length <= 0 {
		return 0.9
	}
	return messageCarriage / length
}

// railGauge sets the rails under the wagon's body, a little over half
// its width apart, the way a real train sits on its track.
func railGauge() float64 {
	return 0.55 * vehicleFeet()[kitWagons[0]].Width * city.CellSize
}

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
	var sleepers, rails []city.Point
	seg := func(path *[]city.Point, a, b city.Point) {
		*path = append(*path, cam.WorldToScreen(a), cam.WorldToScreen(b))
	}
	for i := 1; i < len(c.Rails); i++ {
		a, b := c.Rails[i-1], c.Rails[i]
		dx, dy := b.X-a.X, b.Y-a.Y
		length := math.Hypot(dx, dy)
		if length == 0 {
			continue
		}
		ux, uy := dx/length, dy/length
		nx, ny := -uy*railGauge()/2, ux*railGauge()/2
		for d := sleeperStep / 2; d < length; d += sleeperStep {
			p := city.Point{X: a.X + ux*d, Y: a.Y + uy*d}
			seg(&sleepers, city.Point{X: p.X + nx*1.6, Y: p.Y + ny*1.6}, city.Point{X: p.X - nx*1.6, Y: p.Y - ny*1.6})
		}
		seg(&rails, city.Point{X: a.X + nx, Y: a.Y + ny}, city.Point{X: b.X + nx, Y: b.Y + ny})
		seg(&rails, city.Point{X: a.X - nx, Y: a.Y - ny}, city.Point{X: b.X - nx, Y: b.Y - ny})
	}
	stroke := func(segs []city.Point, w float32, col color.NRGBA) {
		for i := 0; i+1 < len(segs); i += 2 {
			g.paint.AddLine(segs[i], segs[i+1], float64(w), col)
		}
		g.paint.Flush(screen)
	}
	stroke(sleepers, width*1.6, colorSleeper)
	stroke(rails, width, colorRail)
}

// carriage is one piece of a train on the loop this frame.
type carriage struct {
	at    city.Point
	model string
	turn  int
	train *city.Train
	head  bool
	// along is how far round the loop it stands, and loop the loop's
	// length, so a test can measure the spacing the way the train runs.
	along, loop float64
}

// carriages places every train: heads spaced round the loop, each piece
// trailing the one in front by half of each one's length and a coupling,
// all moving at trainSpeed. Each train runs ahead of the one before it,
// trainGap wagons clear of it.
func (g *Game) carriages(c *city.City, seconds float64) []carriage {
	length := pathLength(c.Rails)
	if length <= 0 || len(c.Trains) == 0 {
		return nil
	}
	clear := trainGap * pieceLength(kitWagons[0])
	var out []carriage
	offset, front := 0.0, 0.0
	for i, t := range c.Trains {
		models := make([]string, t.Wagons+1)
		models[0] = kitEngines[t.Engine()]
		for w := 1; w <= t.Wagons; w++ {
			models[w] = kitWagons[t.Hue%len(kitWagons)]
		}
		backs := make([]float64, len(models))
		for w := 1; w < len(models); w++ {
			backs[w] = backs[w-1] + (pieceLength(models[w-1])+pieceLength(models[w]))/2 + coupling
		}
		if i > 0 {
			offset += front + clear + backs[len(backs)-1] + pieceLength(models[len(models)-1])/2
		}
		front = pieceLength(models[0]) / 2
		head := math.Mod(seconds*trainSpeed+offset, length)
		for w, model := range models {
			d := math.Mod(head-backs[w]+length*4, length)
			at, dir := pointAlong(c.Rails, d)
			out = append(out, carriage{at: at, model: model, turn: vehicleTurn(dir), train: t, head: w == 0, along: d, loop: length})
		}
	}
	return out
}

// vehicleTurn is the turn that runs any vehicle nose first along d: an
// engine or wagon, a car or truck, a tug. Every one is modelled nose
// along the kits' -y, which the map draws unturned heading +y.
// TestEngineRunsNoseFirst and TestVehicleRunsNoseFirst hold it against
// the nose the atlas records (items 147, 151).
func vehicleTurn(d city.Point) int {
	switch {
	case math.Abs(d.Y) >= math.Abs(d.X) && d.Y >= 0:
		return 0
	case math.Abs(d.Y) >= math.Abs(d.X):
		return 180
	case d.X >= 0:
		return 90
	default:
		return 270
	}
}

func (g *Game) drawCarriage(screen *ebiten.Image, cam *city.Camera, k carriage) {
	r := g.kit(screen, cam, k.model, k.turn, k.at, nil)
	g.noteHit(r, city.Hit{Train: k.train})
}
