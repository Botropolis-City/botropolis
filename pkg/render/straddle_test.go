package render

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 259: a hull long enough to reach past a raised deck's edge is cut
// where each edge crosses its centreline, and each piece sorts on its
// own slice of the hull, so the deck and its piers draw over the part
// under them and nothing of the deck over the part in front.
func TestABoatStraddlingADeck(t *testing.T) {
	start := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := 0; i < 17; i++ {
		sessions = append(sessions, state.Session{ID: fmt.Sprintf("s%02d", i), CWD: fmt.Sprintf("/p/d%02d", i), State: state.Working, Alive: true, StartedAt: start})
	}
	c := city.Build(state.Snapshot{At: start, Sessions: sessions}, city.NewLayout())
	bridges := c.Bridges()
	require.Len(t, bridges, 2, "precondition: the city has two bridges")
	b := bridges[1]
	require.NotEqual(t, city.Span{}, b.Channel, "precondition: the second bridge crosses the river")
	band := b.Abutment.Rect()
	from, to, width, ok := c.GaugeRun(gaugeEndroom)
	require.True(t, ok, "precondition: the river has a run for the boats")
	g := &Game{kits: preparedKits(t)}

	// The boats are listed here, with the share of the hull's length its
	// centre stands off the deck's centre, toward the camera: a boat
	// mostly near, centred and mostly far, as the liner was at 55%, 61%
	// and 64% in item 252's frames.
	boats := []struct {
		name   string
		piece  string
		extent float64
		lane   int
		offs   []float64
	}{
		{"the liner", kitGaugeLiner, gaugeHulls[city.GaugeBig].extent, 0, []float64{1.0 / 3, 0, -1.0 / 3}},
		{"a courier", kitCourier, courierExtent, 1, []float64{0.4, -0.4}},
	}
	for _, boat := range boats {
		for _, heading := range []int{0, 90, 180, 270} {
			cam := city.NewCamera()
			cam.Projection, cam.Zoom, cam.Heading = city.Isometric, 1, heading
			along := gaugeUpstream(cam, from, to)
			turn := bowTurn(along)
			lane := gaugeLanes(width, 3)[boat.lane]
			x := gaugeTrackAt(cam, from, to, lane, 0.5, 0, 0).X
			centre := city.Point{X: x, Y: band.Center().Y}
			_, _, _, drawn, ok := g.kitSizedPlace(cam, boat.piece, turn, centre, boat.extent)
			require.True(t, ok, "precondition: %s has a sprite", boat.name)
			length := hullLength(cam, centre, along, drawn)
			toward := 1.0
			if !nearer(cam, centre.Add(city.Point{Y: 1}), centre) {
				toward = -1
			}

			for _, off := range boat.offs {
				at := centre.Add(city.Point{Y: toward * off * length})
				_, origin, scale, drawn, _ := g.kitSizedPlace(cam, boat.piece, turn, at, boat.extent)
				cuts := hullCuts(cam, at, along, [2]float64{drawn.Min.X, drawn.Max.X}, bridges)
				t.Run(fmt.Sprintf("when %s stands %+.2f of its length toward the camera off the deck's centre, at heading %d", boat.name, off, heading), func(t *testing.T) {
					reaches := 0
					for _, edge := range []float64{band.Min.Y, band.Max.Y} {
						if lo, hi := hullSpan(at, along, length); edge > lo && edge < hi {
							reaches++
						}
					}
					require.NotZero(t, reaches, "precondition: the hull reaches past a deck edge")

					t.Run("it should cut the hull into one piece per side of each deck edge it reaches past", func(t *testing.T) {
						assert.Len(t, cuts, reaches+1)
					})

					t.Run("it should give the pieces the hull's drawn columns exactly once", func(t *testing.T) {
						var got [][2]int
						for _, p := range cuts {
							lo, hi := pieceColumns(origin.X, scale, drawnColumns(drawn, scale), p.from, p.to)
							got = append(got, [2]int{lo, hi})
						}
						slices.SortFunc(got, func(a, b [2]int) int { return a[0] - b[0] })
						var want [][2]int
						last := 0
						for _, r := range got {
							want = append(want, [2]int{last, r[1]})
							last = r[1]
						}
						if len(want) > 0 {
							want[len(want)-1][1] = drawnColumns(drawn, scale)
						}
						assert.True(t, len(got) > 0 && slices.Equal(want, got), "pieces %v of %d columns", got, drawnColumns(drawn, scale))
					})

					got := namedOrder(cam, func(rec func(string) func()) []drawable {
						var items []drawable
						for i, p := range bridgeItems(cam, b, func(bridgePart) func() { return func() {} }) {
							name := fmt.Sprintf("deck %d", i)
							p.draw = rec(name)
							items = append(items, p)
						}
						for i, p := range cuts {
							name := fmt.Sprintf("hull %d", i)
							d := footprintAt(cam, p.foot, rec(name))
							d.moves = true
							items = append(items, d)
						}
						return items
					})
					deck := bridgeItems(cam, b, func(bridgePart) func() { return func() {} })
					for i, p := range cuts {
						stretch := stretchOf(cam, at, along, p)
						hull := footprintAt(cam, stretch, nil)
						where := "under"
						switch {
						case stretch.Min.Y >= band.Max.Y-1e-6 || stretch.Max.Y <= band.Min.Y+1e-6:
							where = "far"
							if nearer(cam, stretch.Center(), city.Point{X: stretch.Center().X, Y: band.Center().Y}) {
								where = "near"
							}
						}
						for j, d := range deck {
							if hull.right <= d.left || d.right <= hull.left {
								continue
							}
							if where == "under" && !overlapsTurned(hull.turned, d.turned) {
								continue
							}
							deckAt, hullAt := slices.Index(got, fmt.Sprintf("deck %d", j)), slices.Index(got, fmt.Sprintf("hull %d", i))
							switch where {
							case "near":
								t.Run(fmt.Sprintf("it should draw the piece in front of the near edge after deck part %d, which shares its columns", j), func(t *testing.T) {
									assert.Greater(t, hullAt, deckAt, "%v", got)
								})
							case "under":
								t.Run(fmt.Sprintf("it should draw the piece under the deck before span %d over it", j), func(t *testing.T) {
									assert.Less(t, hullAt, deckAt, "%v", got)
								})
							case "far":
								t.Run(fmt.Sprintf("it should draw the piece beyond the far edge before deck part %d, which shares its columns", j), func(t *testing.T) {
									assert.Less(t, hullAt, deckAt, "%v", got)
								})
							}
						}
					}
				})
			}
		}
	}

	t.Run("when a boat is clear of every deck", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Projection, cam.Zoom = city.Isometric, 1
		along := gaugeUpstream(cam, from, to)
		at := gaugeTrackAt(cam, from, to, gaugeLanes(width, 3)[0], 0.5, 0, 0)
		at.Y = band.Center().Y + 6*city.CellSize
		items := g.hullItems(nil, cam, hullDraw{piece: kitGaugeLiner, turn: bowTurn(along), at: at, along: along, extent: gaugeHulls[city.GaugeBig].extent}, bridges)

		t.Run("it should be one drawable", func(t *testing.T) {
			assert.Len(t, items, 1)
		})
	})
}

// hullSpan is how far along the world's y axis a hull of a length
// reaches, centred at a point and lying along a direction.
func hullSpan(at, along city.Point, length float64) (lo, hi float64) {
	a, b := at.Y-along.Y*length/2, at.Y+along.Y*length/2
	return math.Min(a, b), math.Max(a, b)
}

// stretchOf is the stretch of a hull a piece's screen columns cover,
// read back along its centreline, a cell's half-tile either side of it:
// what the piece is, worked out without its own footprint.
func stretchOf(cam *city.Camera, at, along city.Point, p hullPiece) city.Rect {
	sx := func(t float64) float64 { return cam.WorldToScreen(at.Add(along.Scale(t))).X }
	step := sx(1) - sx(0)
	y := func(col float64) float64 { return at.Y + (col-sx(0))/step*along.Y }
	half := city.Tile / 2
	return city.Rect{
		Min: city.Point{X: at.X - half, Y: math.Min(y(p.from), y(p.to))},
		Max: city.Point{X: at.X + half, Y: math.Max(y(p.from), y(p.to))},
	}
}

// drawnColumns is how many source columns a sprite drawn at a scale
// spans.
func drawnColumns(drawn city.Rect, scale float64) int {
	return int(math.Round((drawn.Max.X - drawn.Min.X) / scale))
}
