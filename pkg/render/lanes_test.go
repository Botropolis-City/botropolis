package render

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

var laneWays = map[string]city.Point{"+x": {X: 1}, "-x": {X: -1}, "+y": {Y: 1}, "-y": {Y: -1}}

// sideOf is how far p stands to the right of the line through on along
// d, negative on its left: right is the driver's right with y south.
func sideOf(on, d, p city.Point) float64 {
	n := math.Hypot(d.X, d.Y)
	return ((p.X-on.X)*-d.Y + (p.Y-on.Y)*d.X) / n
}

// steps drives a lane in small steps and returns the furthest any one
// step moved, beside how far each step drove along the lane.
func steps(lane []city.Point, n int) (longest, drove float64) {
	length := pathLength(lane)
	drove = length / float64(n)
	prev, _ := pointAlong(lane, 0)
	for i := 1; i <= n; i++ {
		at, _ := pointAlong(lane, float64(i)*drove)
		longest = math.Max(longest, math.Hypot(at.X-prev.X, at.Y-prev.Y))
		prev = at
	}
	return longest, drove
}

// Item 153: every road is two-way and every road vehicle keeps to the
// lane on the right of its travel.
func TestKeepRight(t *testing.T) {
	for name, d := range laneWays {
		t.Run("when a vehicle travels a straight road along "+name, func(t *testing.T) {
			from := city.Point{X: 100, Y: 200}
			to := from.Add(d.Scale(5 * city.CellSize))
			at, dir := pointAlong(keepRight([]city.Point{from, to}), 2*city.CellSize)

			t.Run("it should sit half a lane from the centre line", func(t *testing.T) {
				assert.InDelta(t, laneOffset, math.Abs(sideOf(from, d, at)), 1e-9)
			})
			t.Run("it should sit on the right of its travel", func(t *testing.T) {
				assert.Greater(t, sideOf(from, d, at), 0.0)
			})
			t.Run("it should still face its travel", func(t *testing.T) {
				assert.Equal(t, vehicleTurn(d), vehicleTurn(dir))
			})
			for _, heading := range noseHeadings {
				t.Run(fmt.Sprintf("and the camera faces %d, it should sit on the driver's right on screen", heading), func(t *testing.T) {
					cam := city.NewCamera()
					cam.Projection = city.Isometric
					cam.Heading = heading
					a, b, p := cam.WorldToScreen(from), cam.WorldToScreen(to), cam.WorldToScreen(at)
					assert.Greater(t, sideOf(a, b.Sub(a), p), 0.0)
				})
			}
		})
	}

	t.Run("when two vehicles go opposite ways on one road", func(t *testing.T) {
		from, to := city.Point{}, city.Point{X: 5 * city.CellSize}
		there, _ := pointAlong(keepRight([]city.Point{from, to}), 2.5*city.CellSize)
		back, _ := pointAlong(keepRight([]city.Point{to, from}), 2.5*city.CellSize)
		apart := math.Abs(there.Y - back.Y)

		t.Run("it should keep their centres one lane apart", func(t *testing.T) {
			assert.InDelta(t, 2*laneOffset, apart, 1e-9)
		})
		t.Run("it should keep them further apart than the widest road vehicle is wide", func(t *testing.T) {
			atlas := noseAtlas(t)
			widest := 0.0
			for _, piece := range append(append([]string{}, kitCars...), kitVan, kitDelivery, kitInspector, kitTruck) {
				for _, heading := range noseHeadings {
					s, ok := atlas.Sprite(piece, heading)
					require.True(t, ok, piece)
					widest = math.Max(widest, s.Foot.Width)
				}
			}
			require.Positive(t, widest)
			assert.Greater(t, apart, widest*city.CellSize)
		})
	})

	t.Run("when the lane is held against the road the atlas draws", func(t *testing.T) {
		atlas := noseAtlas(t)
		s, ok := atlas.Sprite("city-kit-roads/road-straight", 0)
		require.True(t, ok)
		page := atlas.Pages[s.Page]
		half := atlas.Tile / 2
		for name, side := range map[string]float64{"right of +x": 1, "right of -x": -1} {
			// road-straight unturned runs along x; a lane centre is
			// laneOffset across it, along y, projected at heading 0.
			y := side * laneOffset / city.CellSize
			px := s.Rect.Min.X + s.Anchor.X + int(math.Round(-y*half))
			py := s.Rect.Min.Y + s.Anchor.Y + int(math.Round(y*half/2))
			c := color.NRGBAModel.Convert(page.At(px, py)).(color.NRGBA)
			asphalt := c.A > 200 && (int(c.R)+int(c.G)+int(c.B))/3 < 200 && int(c.B) < int(c.R)+40

			t.Run("it should put the lane "+name+" on asphalt, between the kerb and the lane line", func(t *testing.T) {
				assert.True(t, asphalt, "pixel at the lane centre is %v", c)
			})
		}
	})

	corners := map[string][]city.Point{
		"left":  {{X: 0, Y: 0}, {X: 3 * city.CellSize, Y: 0}, {X: 3 * city.CellSize, Y: -3 * city.CellSize}},
		"right": {{X: 0, Y: 0}, {X: 3 * city.CellSize, Y: 0}, {X: 3 * city.CellSize, Y: 3 * city.CellSize}},
	}
	for name, path := range corners {
		t.Run("when a vehicle turns "+name+" at a corner", func(t *testing.T) {
			lane := keepRight(path)
			length := pathLength(lane)
			before, _ := pointAlong(lane, 0.2*length)
			after, _ := pointAlong(lane, 0.8*length)

			t.Run("it should keep right before the corner", func(t *testing.T) {
				assert.InDelta(t, laneOffset, sideOf(path[0], path[1].Sub(path[0]), before), 1e-9)
			})
			t.Run("it should keep right after the corner", func(t *testing.T) {
				assert.InDelta(t, laneOffset, sideOf(path[1], path[2].Sub(path[1]), after), 1e-9)
			})
			t.Run("it should never move further in a step than it drove", func(t *testing.T) {
				longest, drove := steps(lane, 400)
				assert.LessOrEqual(t, longest, drove+1e-9)
			})
		})
	}

	t.Run("when a vehicle drives a road there and back", func(t *testing.T) {
		path := []city.Point{{X: 0, Y: 0}, {X: 3 * city.CellSize, Y: 0}, {X: 3 * city.CellSize, Y: 3 * city.CellSize}}
		loop := keepRightLoop(thereAndBack(path))

		t.Run("it should never move further in a step than it drove", func(t *testing.T) {
			longest, drove := steps(loop, 400)
			assert.LessOrEqual(t, longest, drove+1e-9)
		})
		t.Run("it should come round to where it started", func(t *testing.T) {
			assert.Equal(t, loop[0], loop[len(loop)-1])
		})
	})
}
