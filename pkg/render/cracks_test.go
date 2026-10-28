package render

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 315 (Aria, 2026-10-28: "Yes, take S4"): below a quarter zoom a
// tile is under 4 pixels, and a crack on a lot or a district's floor is a
// speck a few pixels long, stroked as a mesh: the six-hundred-district
// city's first frame spent 0.3-0.4 CPU-seconds on them. There, none are
// stroked.
func TestCracksAreStrokedOnlyWhereTheyShow(t *testing.T) {
	rect := city.Rect{Max: city.Point{X: 20 * city.Tile, Y: 20 * city.Tile}}
	cracked := districtGround{Wear: 0.5, Cracks: [][]city.Point{
		{{X: 30, Y: 30}, {X: 40, Y: 34}, {X: 52, Y: 31}},
		{{X: 120, Y: 90}, {X: 128, Y: 99}},
	}}
	bare := districtGround{Wear: 0.5}
	// recorded is how many commands a district floor records at zoom.
	recorded := func(ground districtGround, zoom float64) int {
		g := &Game{}
		l := g.recorder()
		l.begin()
		cam := city.NewCamera()
		cam.Zoom = zoom
		g.districtFloor(l.dry, cam, rect, ground)
		got := l.end()
		return got.to - got.from
	}

	for _, c := range []struct {
		zoom    float64
		strokes bool
	}{{0.1, false}, {0.249, false}, {0.25, true}, {1, true}} {
		t.Run(fmt.Sprintf("when a cracked district floor is drawn at zoom %v", c.zoom), func(t *testing.T) {
			withCracks, without := recorded(cracked, c.zoom), recorded(bare, c.zoom)

			t.Run(fmt.Sprintf("it should stroke its cracks: %v", c.strokes), func(t *testing.T) {
				assert.Equal(t, c.strokes, withCracks > without)
			})
		})
	}
}
