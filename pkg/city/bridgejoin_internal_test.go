package city

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
)

// Item 108, Aria 2026-10-18: a road that reaches the bridge's landing
// joins the bridge with a T, rather than running past its end.
func TestRoadJoinsTheBridge(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		sessions = append(sessions, state.Session{ID: name, CWD: "/p/" + name, State: state.Working, StartedAt: at})
	}
	c := Build(state.Snapshot{At: at, Sessions: sessions, Roads: []state.Road{{From: "/p/a", To: "/p/b", Files: 1, Sessions: 1}}}, NewLayout())
	land, ok := c.bridgeCell()
	require.True(t, ok, "precondition: the city has a bridge")
	toward, along := DirE, -1
	if c.Bridge().Far.X < c.Bridge().Land.X {
		toward, along = DirW, 1
	}
	require.True(t, c.IsStreet(Cell{land.Col, land.Row - 1}) && c.IsStreet(Cell{land.Col, land.Row + 1}), "precondition: the landing is on an avenue running north and south")

	// A road down the avenue past the landing, from one lot beside it to
	// another.
	path := []Point{Cell{land.Col + along, land.Row - 2}.Center()}
	for row := land.Row - 2; row <= land.Row+2; row++ {
		path = append(path, Cell{land.Col, row}.Center())
	}
	path = append(path, Cell{land.Col + along, land.Row + 2}.Center())
	c.Streets = []Street{{Road: &c.Roads[0], Path: path}}
	c.StreetCells = nil
	c.Lamps = append([]Cell(nil), c.planLamps...)
	c.Trees = append([]Tree(nil), c.planTrees...)
	c.layRoads()
	mask := -1
	for _, sc := range c.StreetCells {
		if sc.Cell == land {
			mask = sc.Mask
		}
	}
	require.NotEqual(t, -1, mask, "precondition: the road is laid on the landing")

	t.Run("when a road runs past the bridge's landing", func(t *testing.T) {
		t.Run("it should join the bridge", func(t *testing.T) {
			assert.NotZero(t, mask&toward)
		})

		t.Run("it should lay no driveway pad onto the bridge", func(t *testing.T) {
			assert.Empty(t, c.Pads([]StreetCell{{Cell: land, Mask: mask}}))
		})

		// Item 247: the bridge's own road is laid only where no road
		// already reaches the landing.
		t.Run("it should lay no road but the road's own", func(t *testing.T) {
			var laid []Cell
			for _, sc := range c.StreetCells {
				laid = append(laid, sc.Cell)
			}
			var own []Cell
			for _, p := range path[1 : len(path)-1] {
				own = append(own, CellOf(p))
			}
			assert.ElementsMatch(t, own, laid)
		})
	})
}
