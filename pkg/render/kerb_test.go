package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// siteTruckFoot is the skip truck's and the mixer's footprint, in cells,
// as tools/render-sprites/buildings.py models them: a 0.13 chassis with
// its wheels standing out, 0.42 long. The pipeline records no foot for
// its own pieces.
var siteTruckFoot = assets.Foot{Width: 0.14, Length: 0.42}

// wheels is the four corners of a vehicle's footprint standing at a
// place and facing dir.
func wheels(at, dir city.Point, foot assets.Foot, scale float64) [4]city.Point {
	n := math.Hypot(dir.X, dir.Y)
	if n == 0 {
		dir, n = city.Point{X: 1}, 1
	}
	along := dir.Scale(foot.Length * city.CellSize * scale / 2 / n)
	across := city.Point{X: -dir.Y, Y: dir.X}.Scale(foot.Width * city.CellSize * scale / 2 / n)
	return [4]city.Point{at.Add(along).Add(across), at.Add(along).Sub(across), at.Sub(along).Sub(across), at.Sub(along).Add(across)}
}

// ground says whether a world point is somewhere a road vehicle's wheel
// may stand: a lot's floor, or the carriageway of a laid road cell as
// the atlas draws its piece — asphalt, the gutter or the lane line,
// never the pale kerb and never past the road. Along a seam the piece
// next door draws too, so every piece that reaches the point is read.
type ground struct {
	c     *city.City
	atlas assets.KitAtlas
	cells map[city.Cell]int
}

func newGround(t *testing.T, c *city.City) ground {
	g := ground{c: c, atlas: noseAtlas(t), cells: map[city.Cell]int{}}
	for _, sc := range c.StreetCells {
		g.cells[sc.Cell] = sc.Mask
	}
	return g
}

func (g ground) carriageway(p city.Point) bool {
	home := city.CellOf(p)
	road := false
	for _, cell := range []city.Cell{home, {Col: home.Col + 1, Row: home.Row}, {Col: home.Col - 1, Row: home.Row}, {Col: home.Col, Row: home.Row + 1}, {Col: home.Col, Row: home.Row - 1}} {
		c, ok := g.pixel(cell, p)
		switch {
		case !ok || c.A <= 64:
		case min(c.R, c.G) >= 180:
			return false
		default:
			road = true
		}
	}
	return road
}

// pixel is the colour the road piece laid on cell draws at p, which a
// neighbour's piece reaches only along their seam.
func (g ground) pixel(cell city.Cell, p city.Point) (color.NRGBA, bool) {
	mask, laid := g.cells[cell]
	if !laid {
		return color.NRGBA{}, false
	}
	name, turn := roadPiece(mask)
	s, ok := g.atlas.Sprite(name, -turn)
	if !ok {
		return color.NRGBA{}, false
	}
	d := p.Sub(cell.Center()).Scale(1 / city.CellSize)
	half := g.atlas.Tile / 2
	at := image.Pt(s.Rect.Min.X+int(math.Round(float64(s.Anchor.X)+(d.X-d.Y)*half)), s.Rect.Min.Y+int(math.Round(float64(s.Anchor.Y)+(d.X+d.Y)*half/2)))
	if !at.In(s.Rect) {
		return color.NRGBA{}, false
	}
	return color.NRGBAModel.Convert(g.atlas.Pages[s.Page].At(at.X, at.Y)).(color.NRGBA), true
}

// slack is how far a wheel may be from its ground and still stand on
// it: about a pixel at zoom 2, where a tile's edge pixel against a lot
// is too faint to read, and a tenth of the kerb's width.
const slack = 0.5

func (g ground) holds(p city.Point) bool {
	for _, d := range []city.Point{{}, {X: slack}, {X: -slack}, {Y: slack}, {Y: -slack}} {
		if q := p.Add(d); g.c.InBlock(q) || g.carriageway(q) {
			return true
		}
	}
	return false
}

// offGround is every wheel of a vehicle placed at each of places that
// stands off its ground.
func (g ground) offGround(places []placed, foot assets.Foot, scale float64) []city.Point {
	var off []city.Point
	for _, p := range places {
		for _, w := range wheels(p.at, p.dir, foot, scale) {
			if !g.holds(w) {
				off = append(off, w)
			}
		}
	}
	return off
}

type placed struct{ at, dir city.Point }

// drive is a vehicle's places at n even steps from 0 to span.
func drive(n int, span float64, place func(float64) (city.Point, city.Point)) []placed {
	out := make([]placed, 0, n+1)
	for i := 0; i <= n; i++ {
		at, dir := place(span * float64(i) / float64(n))
		out = append(out, placed{at, dir})
	}
	return out
}

// kerbCity is six lots with roads between them, its streets placed for
// the camera at a heading, so each heading puts the driveways on another
// side of their lots.
func kerbCity(heading int) *city.City {
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		for k := 0; k < 3; k++ {
			sessions = append(sessions, state.Session{ID: fmt.Sprintf("%s%d", name, k), CWD: "/p/" + name, State: state.Working, StartedAt: at.Add(time.Duration(k) * time.Minute)})
		}
	}
	var roads []state.Road
	for _, p := range [][2]string{{"a", "d"}, {"b", "e"}, {"c", "f"}} {
		roads = append(roads, state.Road{From: "/p/" + p[0], To: "/p/" + p[1], Messages: 1, Sessions: 1, Active: 1})
	}
	c := city.Build(state.Snapshot{At: at, Sessions: sessions, Roads: roads}, city.NewLayout())
	c.Face(heading)
	return c
}

// leaving is a street that leaves a district, driven from it.
func leaving(c *city.City, d *city.District) ([]city.Point, bool) {
	for _, st := range c.Streets {
		switch d {
		case st.Road.From:
			return st.Path, true
		case st.Road.To:
			out := make([]city.Point, len(st.Path))
			for i, p := range st.Path {
				out[len(st.Path)-1-i] = p
			}
			return out, true
		}
	}
	return nil, false
}

// drivewaySide is which side of its lot a driveway is on.
func drivewaySide(d *city.District, dw city.Driveway) int {
	p := dw.Cell.Center()
	switch {
	case p.Y > d.Rect.Max.Y:
		return city.DirS
	case p.Y < d.Rect.Min.Y:
		return city.DirN
	case p.X > d.Rect.Max.X:
		return city.DirE
	}
	return city.DirW
}

// Item 222: every road vehicle's wheels stay on its ground — the lot's
// floor inside a block and the carriageway on a road, never the kerb —
// whichever side of its lot the driveway is on.
func TestRoadVehiclesStayOnTheirGround(t *testing.T) {
	t0 := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	const n = 300
	feet := vehicleFeet()
	cam := city.NewCamera()
	cam.Projection = city.Isometric
	g := &Game{}

	sides := map[int]bool{}
	for _, heading := range noseHeadings {
		c := kerbCity(heading)
		on := newGround(t, c)
		require.NotEmpty(t, c.Streets, "precondition: the city has streets")
		for _, d := range c.Districts {
			if dw, ok := c.Driveway(d); ok {
				sides[drivewaySide(d, dw)] = true
			}
		}

		for _, d := range c.Districts {
			street, ok := leaving(c, d)
			if !ok {
				continue
			}
			for _, b := range d.Buildings {
				route := d.Route(b)
				end := route[len(route)-1]
				park := city.Point{X: end.X - city.Tile*0.6, Y: end.Y}
				inspector := func(leaving bool) []placed {
					return drive(n, 1, func(p float64) (city.Point, city.Point) {
						return inspectorPlace(city.Inspector{Park: park, Parked: p, Leaving: leaving})
					})
				}
				delivery := city.Delivery{Path: append([]city.Point{end}, street...), Since: t0}
				corner, sides := lotCorner(cam, b)
				pour := corner.Add(sides[0].Sub(corner).Scale(0.5))
				site := func(stop city.Point) []placed {
					in, out := g.jobPaths(c, b, stop)
					return drive(n, 2, func(along float64) (city.Point, city.Point) {
						return jobPlace(in, out, along, c.InBlock)
					})
				}

				for _, v := range []struct {
					name   string
					places []placed
					foot   assets.Foot
					scale  float64
				}{
					{"the inspector's car arriving", inspector(false), feet[kitInspector], 0.8},
					{"the inspector's car leaving", inspector(true), feet[kitInspector], 0.8},
					{"the delivery truck", drive(n, 1, func(p float64) (city.Point, city.Point) {
						return deliveryPlace(delivery, t0.Add(time.Duration(p*float64(delivery.For()))), c.InBlock)
					}), feet[kitDelivery], 1},
					{"the skip truck", site(corner), siteTruckFoot, 1},
					{"the mixer", site(pour), siteTruckFoot, 1},
				} {
					t.Run(fmt.Sprintf("when %s drives from %s's %s with the camera at %d", v.name, d.Name, b.Session.ID, heading), func(t *testing.T) {
						t.Run("it should keep every wheel on the lot's floor or the carriageway", func(t *testing.T) {
							assert.Empty(t, on.offGround(v.places, v.foot, v.scale))
						})
					})
				}

				t.Run(fmt.Sprintf("when a site truck drives between the buildings to %s with the camera at %d", b.Session.ID, heading), func(t *testing.T) {
					var through []city.Point
					for _, p := range append(site(corner), site(pour)...) {
						for _, w := range wheels(p.at, p.dir, siteTruckFoot, 1) {
							for _, other := range d.Buildings {
								if other != b && other.Rect.Inset(city.Tile/2).Contains(w) {
									through = append(through, w)
								}
							}
						}
					}

					t.Run("it should keep its wheels clear of the buildings beside its own", func(t *testing.T) {
						assert.Empty(t, through)
					})
				})
			}
		}

		for i, st := range c.Streets {
			m := city.Message{Path: st.Path, Since: t0}
			for _, v := range []struct {
				name   string
				places []placed
				foot   assets.Foot
			}{
				{"a traffic car", drive(n, 1, func(p float64) (city.Point, city.Point) { return carPlace(st.Path, p, c.InBlock) }), feet[kitCars[0]]},
				{"a message van", drive(n, 1, func(p float64) (city.Point, city.Point) {
					return messagePlace(m, t0.Add(time.Duration(p*float64(m.For()))), c.InBlock)
				}), feet[kitVan]},
			} {
				t.Run(fmt.Sprintf("when %s drives street %d with the camera at %d", v.name, i, heading), func(t *testing.T) {
					t.Run("it should keep every wheel on the lot's floor or the carriageway", func(t *testing.T) {
						assert.Empty(t, on.offGround(v.places, v.foot, 1))
					})
				})
			}
		}
	}

	t.Run("when the lots are laid out, their driveways should face every way", func(t *testing.T) {
		assert.Len(t, sides, 4)
	})

	t.Run("when a lot's lane meets an avenue's at a corner", func(t *testing.T) {
		lot := city.Rect{Min: city.Point{X: 0, Y: 0}, Max: city.Point{X: 6 * city.CellSize, Y: 6 * city.CellSize}}
		inLot := func(p city.Point) bool { return lot.Contains(p) }
		path := []city.Point{
			{X: 2 * city.CellSize, Y: lot.Max.Y - city.FrontLane},
			{X: 3.5 * city.CellSize, Y: lot.Max.Y - city.FrontLane},
			{X: 3.5 * city.CellSize, Y: lot.Max.Y + city.CellSize/2},
			{X: 8 * city.CellSize, Y: lot.Max.Y + city.CellSize/2},
		}
		for name, p := range map[string][]city.Point{"driving out": path, "driving in": thereAndBack(path)[len(path)-1:]} {
			t.Run("and the vehicle is "+name, func(t *testing.T) {
				lane := lane(p, false, inLot)

				t.Run("it should never move further in a step than it drove", func(t *testing.T) {
					longest, drove := steps(lane, 400)
					assert.LessOrEqual(t, longest, drove+1e-9)
				})
			})
		}

		t.Run("it should keep a lot lane's half-lane inside the lot", func(t *testing.T) {
			at, _ := pointAlong(lane(path, false, inLot), 0.5*city.CellSize)
			assert.InDelta(t, lotLaneOffset, sideOf(path[0], path[1].Sub(path[0]), at), 1e-9)
		})

		t.Run("it should keep a road's half-lane on the avenue", func(t *testing.T) {
			l := lane(path, false, inLot)
			at, _ := pointAlong(l, pathLength(l)-city.CellSize)
			assert.InDelta(t, laneOffset, sideOf(path[2], path[3].Sub(path[2]), at), 1e-9)
		})
	})
}

// Item 222: the zoom was a suspect — a sprite's anchor that does not
// scale with the level drawn would put a car right at fit and wrong at
// zoom 2. Every road vehicle stands on the same ground point at every
// level: each level's anchor is a whole pixel of its own, so they differ
// by rounding, which is a small part of a world unit on the ground.
func TestVehicleSpritesStandOnTheirFoot(t *testing.T) {
	const within = 2.0 // world units, half the gutter beside a lane
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(atlases), 2, "precondition: more than one level")
	coarse := atlases[0]
	for _, piece := range append(append([]string{}, kitCars...), kitVan, kitDelivery, kitInspector, kitTruck, kitSkipTruck, kitMixer) {
		for _, heading := range noseHeadings {
			for _, fine := range atlases[1:] {
				t.Run(fmt.Sprintf("when %s is drawn at heading %d from the %gx level", piece, heading, fine.Zoom), func(t *testing.T) {
					a, ok := coarse.Sprite(piece, heading)
					require.True(t, ok)
					b, ok := fine.Sprite(piece, heading)
					require.True(t, ok)
					k := coarse.Zoom / fine.Zoom
					half := coarse.Tile / 2
					u := (float64(b.Anchor.X)*k - float64(a.Anchor.X)) / half
					v := (float64(b.Anchor.Y)*k - float64(a.Anchor.Y)) / (half / 2)
					apart := math.Hypot((u+v)/2, (v-u)/2) * city.CellSize

					t.Run(fmt.Sprintf("it should stand within %g world unit of where the coarser level stands it", within), func(t *testing.T) {
						assert.LessOrEqual(t, apart, within)
					})
				})
			}
		}
	}
}
