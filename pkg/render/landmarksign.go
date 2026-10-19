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
	Sign ui.Sign
	Side string
	Box  wallBox
	Quad [4]city.Point
	Ink  color.NRGBA
}

// landmarkLettering is a landmark's name in capitals across the clear
// band measured on one of the walls the camera sees, as large as fits
// it, leaning with the wall. It is left off rather than squeezed when
// the band cannot hold it at a legible size.
func (g *Game) landmarkLettering(cam *city.Camera, lot city.Rect, kit, name string) (lettering, bool) {
	view, ok := g.landmarkView(cam, lot, kit)
	if !ok {
		return lettering{}, false
	}
	spot := view.walls.Sign
	band, side := spot.Band, spot.Side
	text := strings.ToUpper(name)
	tw, th := g.measure(text)
	run := math.Abs(view.at(side, band[1], 0).X - view.at(side, band[0], 0).X)
	tall := (band[3] - band[2]) * view.wall()
	if tw <= 0 || th <= 0 || run <= 0 || tall <= 0 {
		return lettering{}, false
	}
	scale := math.Min(letterFill*run/tw, letterHeight*tall/th)
	if scale*th < ui.MinSignPx {
		return lettering{}, false
	}
	uc, vc := (band[0]+band[1])/2, (band[2]+band[3])/2
	du := scale * tw / 2 / run * (band[1] - band[0])
	dv := scale * th / 2 / view.wall()
	box := wallBox{uc - du, uc + du, vc - dv, vc + dv}
	lean := 0.5
	if side != "left" {
		lean = -0.5
	}
	centre := view.at(side, uc, vc)
	ink := colorKitKerb
	if spot.Dark {
		ink = colorText
	}
	return lettering{
		Sign: ui.Sign{
			Text:  text,
			At:    city.Point{X: centre.X - scale*tw/2, Y: centre.Y - scale*th/2 - lean*scale*tw/2},
			Scale: scale,
			Lean:  lean,
		},
		Side: side,
		Box:  box,
		Quad: view.quad(side, box),
		Ink:  ink,
	}, true
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
func (g *Game) letterLandmark(screen *ebiten.Image, cam *city.Camera, lot city.Rect, kit, name string) {
	if l, ok := g.landmarkLettering(cam, lot, kit, name); ok {
		g.sign(screen, l.Sign, l.Ink)
	}
}

// landmarkPerch is where the plant's chimney stands once the plant is
// turned: plantStackPerch's place, turned with the building about the
// plot's centre, since a chimney is a fixture of its building.
func landmarkPerch(plant city.Rect, side float64) city.Point {
	p, c := plantStackPerch(plant, side), plant.Center()
	return city.Point{X: 2*c.X - p.X, Y: 2*c.Y - p.Y}
}
