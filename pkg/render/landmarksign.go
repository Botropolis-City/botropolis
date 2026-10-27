package render

import (
	_ "embed"
	"encoding/json"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// landmarkTurn is how far the plant, the library and city hall are
// turned in the world, so that their front doors face the camera's home
// heading (item 99; Aria, 2026-10-19).
const landmarkTurn = 180

//go:embed landmarkwalls.json
var landmarkWallsJSON []byte

// wallBox is a rectangle on a wall in the wall's own terms: u across it
// from its left end to its right end on screen, v up it from the ground
// to the roof, each from 0 to 1.
type wallBox [4]float64

func (b wallBox) overlaps(o wallBox) bool {
	return b[0] < o[1] && o[0] < b[1] && b[2] < o[3] && o[2] < b[3]
}

// landmarkWall is one landmark sprite's two seen walls, measured by hand
// from the atlas (tools/landmark-walls draws them back over it), in
// fractions of the sprite so they hold at every zoom's cut.
type landmarkWall struct {
	Near     [2]float64           `json:"near"`
	Left     float64              `json:"left"`
	Right    float64              `json:"right"`
	Wall     float64              `json:"wall"`
	Entrance string               `json:"entrance"`
	Openings map[string][]wallBox `json:"openings"`
	Sign     struct {
		Side string  `json:"side"`
		Band wallBox `json:"band"`
		Dark bool    `json:"dark"`
	} `json:"sign"`
}

// landmarkWalls is a landmark's walls at each of the atlas's headings,
// and a hash of the pixels they were measured from.
type landmarkWalls struct {
	Art      string                  `json:"art"`
	Headings map[string]landmarkWall `json:"headings"`
}

var landmarkTable = func() map[string]landmarkWalls {
	var t map[string]landmarkWalls
	if err := json.Unmarshal(landmarkWallsJSON, &t); err != nil {
		panic("landmarkwalls.json: " + err.Error())
	}
	return t
}()

// landmarkView is a landmark's sprite as drawn this frame, with its
// walls placed on screen.
type landmarkView struct {
	walls  landmarkWall
	origin city.Point
	size   city.Point
	panes  [][4]city.Point
}

// landmarkView places a landmark's measured walls over its sprite as
// kitPlace stands it on its plot, turned, and its window panes with them.
func (g *Game) landmarkView(cam *city.Camera, lot city.Rect, kit string) (landmarkView, bool) {
	table, ok := landmarkTable[kit]
	if !ok || g.kits == nil {
		return landmarkView{}, false
	}
	heading := cam.Heading - landmarkTurn
	walls, ok := table.Headings[strconv.Itoa(((heading/90)%4+4)%4*90)]
	if !ok {
		return landmarkView{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return landmarkView{}, false
	}
	sprite, found := atlas.Sprite(kit, heading)
	_, origin, scale, rect, placed := g.kitPlace(cam, kit, landmarkTurn, lot.Center(), 0)
	if !found || !placed {
		return landmarkView{}, false
	}
	v := landmarkView{walls: walls, origin: rect.Min, size: city.Point{X: rect.Width(), Y: rect.Height()}}
	for _, p := range sprite.Panes {
		var q [4]city.Point
		for i, c := range p {
			q[i] = city.Point{X: origin.X + float64(c.X)*scale, Y: origin.Y + float64(c.Y)*scale}
		}
		v.panes = append(v.panes, q)
	}
	return v, true
}

// at is the screen point u across a wall and w up it.
func (v landmarkView) at(side string, u, w float64) city.Point {
	near := city.Point{X: v.origin.X + v.walls.Near[0]*v.size.X, Y: v.origin.Y + v.walls.Near[1]*v.size.Y}
	var x, ground float64
	if side == "left" {
		from := v.origin.X + v.walls.Left*v.size.X
		x = from + u*(near.X-from)
		ground = near.Y - (near.X-x)/2
	} else {
		to := v.origin.X + v.walls.Right*v.size.X
		x = near.X + u*(to-near.X)
		ground = near.Y - (x-near.X)/2
	}
	return city.Point{X: x, Y: ground - w*v.wall()}
}

// wall is the wall's height on screen.
func (v landmarkView) wall() float64 { return v.walls.Wall * v.size.Y }

// quad is a wall box's corners on screen: top left, top right, bottom
// right, bottom left.
func (v landmarkView) quad(side string, b wallBox) [4]city.Point {
	return [4]city.Point{v.at(side, b[0], b[3]), v.at(side, b[1], b[3]), v.at(side, b[1], b[2]), v.at(side, b[0], b[2])}
}

// lettering is a landmark's name painted on one of its walls.
type lettering struct {
	Signs []ui.Sign
	Side  string
	Box   wallBox
	Quad  [4]city.Point
	Ink   color.NRGBA
}

// landmarkPlot is a landmark standing in the city: its plot, its piece
// and its name.
type landmarkPlot struct {
	lot       city.Rect
	kit, name string
}

// cityLandmarks is every landmark standing in the city.
func cityLandmarks(c *city.City) []landmarkPlot {
	var out []landmarkPlot
	for _, p := range []landmarkPlot{
		{lot: c.Plant.Rect, kit: kitPlant, name: "power plant"},
		{lot: c.Library.Rect, kit: kitLibrary, name: "library"},
		{lot: c.Hall.Rect, kit: kitHall, name: "city hall"},
		{lot: c.Clinic.Rect, kit: kitClinic, name: "clinic"},
	} {
		if p.lot.Area() > 0 {
			out = append(out, p)
		}
	}
	return out
}

// letterCandidate is one place a landmark's name could be painted, and
// how much of it a nearer landmark would hide.
type letterCandidate struct {
	lettering
	covered float64
}

// landmarkLettering is where a landmark's name is painted this frame:
// the first candidate nothing nearer hides, or else the least hidden
// (item 99).
func (g *Game) landmarkLettering(cam *city.Camera, plots []landmarkPlot, i int) (lettering, bool) {
	return pickLettering(g.letterCandidates(cam, plots, i))
}

// letterCandidates are the places a landmark's name could go at this
// heading, most preferred first: its measured band, then each half of
// it, so a name a nearer landmark would cut can stand whole, smaller, on
// the part still seen; and in each, the name on one line and then, when
// it is two words, stacked. A place too small for the name to be
// legible is not a candidate.
func (g *Game) letterCandidates(cam *city.Camera, plots []landmarkPlot, i int) []letterCandidate {
	p := plots[i]
	view, ok := g.landmarkView(cam, p.lot, p.kit)
	if !ok {
		return nil
	}
	spot := view.walls.Sign
	b := spot.Band
	mid := (b[0] + b[1]) / 2
	text := strings.ToUpper(p.name)
	layouts := [][]string{{text}}
	if words := strings.Fields(text); len(words) > 1 {
		layouts = append(layouts, words)
	}
	var out []letterCandidate
	for _, band := range []wallBox{b, {b[0], mid, b[2], b[3]}, {mid, b[1], b[2], b[3]}} {
		for _, lines := range layouts {
			if l, ok := g.letterOn(view, spot.Side, band, spot.Dark, lines); ok {
				out = append(out, letterCandidate{lettering: l, covered: g.letteringCovered(cam, plots, i, l)})
			}
		}
	}
	return out
}

// pickLettering takes the first candidate nothing hides, or else the
// one least hidden.
func pickLettering(candidates []letterCandidate) (lettering, bool) {
	if len(candidates) == 0 {
		return lettering{}, false
	}
	best := candidates[0]
	for _, c := range candidates {
		if c.covered == 0 {
			return c.lettering, true
		}
		if c.covered < best.covered {
			best = c
		}
	}
	return best.lettering, true
}

// letteringCovered is the share of a landmark's lettering that a nearer
// landmark stands in front of, sampled across the lettering.
func (g *Game) letteringCovered(cam *city.Camera, plots []landmarkPlot, i int, l lettering) float64 {
	depth := cam.DepthOf(plots[i].lot)
	var fronts [][6]city.Point
	for j, o := range plots {
		if j == i || cam.DepthOf(o.lot) <= depth {
			continue
		}
		if v, ok := g.landmarkView(cam, o.lot, o.kit); ok {
			fronts = append(fronts, v.silhouette())
		}
	}
	const across, up = 9, 3
	hidden := 0
	for a := range across {
		for u := range up {
			s, t := (float64(a)+0.5)/across, (float64(u)+0.5)/up
			at := lerp(lerp(l.Quad[0], l.Quad[1], s), lerp(l.Quad[3], l.Quad[2], s), t)
			for _, f := range fronts {
				if insideConvex(f[:], at) {
					hidden++
					break
				}
			}
		}
	}
	return float64(hidden) / (across * up)
}

// insideConvex reports whether p lies inside a convex polygon.
func insideConvex(poly []city.Point, p city.Point) bool {
	sign := 0.0
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		cross := (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
		if cross == 0 {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if (sign > 0) != (cross > 0) {
			return false
		}
	}
	return true
}

// silhouette is the drawn landmark's outline from its measured walls:
// its ground corners, the tops of its two seen walls, and the back
// corner of its roof.
func (v landmarkView) silhouette() [6]city.Point {
	left, near, right := v.at("left", 0, 0), v.at("left", 1, 0), v.at("right", 1, 0)
	leftTop, nearTop, rightTop := v.at("left", 0, 1), v.at("left", 1, 1), v.at("right", 1, 1)
	backTop := city.Point{X: leftTop.X + rightTop.X - nearTop.X, Y: leftTop.Y + rightTop.Y - nearTop.Y}
	return [6]city.Point{left, near, right, rightTop, backTop, leftTop}
}

// letterOn is a landmark's name in capitals across one band of one of
// its walls, a line to each of lines, as large as fits it, leaning with
// the wall, or false when the band cannot hold it at a legible size
// rather than squeezing it.
func (g *Game) letterOn(view landmarkView, side string, band wallBox, dark bool, lines []string) (lettering, bool) {
	widest, th := 0.0, 0.0
	widths := make([]float64, len(lines))
	for i, line := range lines {
		w, h := g.measure(line)
		widths[i] = w
		widest, th = math.Max(widest, w), math.Max(th, h)
	}
	n := float64(len(lines))
	run := math.Abs(view.at(side, band[1], 0).X - view.at(side, band[0], 0).X)
	tall := (band[3] - band[2]) * view.wall()
	if widest <= 0 || th <= 0 || run <= 0 || tall <= 0 {
		return lettering{}, false
	}
	scale := math.Min(letterFill*run/widest, letterHeight*tall/(n*th))
	if scale*th < ui.MinSignPx {
		return lettering{}, false
	}
	uc, vc := (band[0]+band[1])/2, (band[2]+band[3])/2
	du := scale * widest / 2 / run * (band[1] - band[0])
	line := scale * th / view.wall()
	box := wallBox{uc - du, uc + du, vc - n*line/2, vc + n*line/2}
	lean := 0.5
	if side != "left" {
		lean = -0.5
	}
	ink := colorKitKerb
	if dark {
		ink = colorText
	}
	l := lettering{Side: side, Box: box, Quad: view.quad(side, box), Ink: ink}
	for i, text := range lines {
		centre := view.at(side, uc, vc+((n-1)/2-float64(i))*line)
		w := widths[i]
		l.Signs = append(l.Signs, ui.Sign{
			Text:  text,
			At:    city.Point{X: centre.X - scale*w/2, Y: centre.Y - scale*th/2 - lean*scale*w/2},
			Scale: scale,
			Lean:  lean,
		})
	}
	return l, true
}

// How much of its band a landmark's lettering may fill: nearly all of
// its run, and less of its height, so the paint has a margin above and
// below.
const (
	letterFill   = 0.9
	letterHeight = 0.8
)

// letterLandmark paints a landmark's name on its wall. It is paint, not
// chrome, so it stays when h hides the interface.
func (g *Game) letterLandmark(screen *ebiten.Image, cam *city.Camera, c *city.City, kit string) {
	plots := cityLandmarks(c)
	for i, p := range plots {
		if p.kit != kit {
			continue
		}
		if l, ok := g.landmarkLettering(cam, plots, i); ok {
			for _, s := range l.Signs {
				g.sign(screen, s, l.Ink)
			}
		}
	}
}

// landmarkPerch is where the plant's chimney stands once the plant is
// turned: plantStackPerch's place, turned with the building about the
// plot's centre, since a chimney is a fixture of its building.
func landmarkPerch(plant city.Rect, side float64) city.Point {
	p, c := plantStackPerch(plant, side), plant.Center()
	return city.Point{X: 2*c.X - p.X, Y: 2*c.Y - p.Y}
}
