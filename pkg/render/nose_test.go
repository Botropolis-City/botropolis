package render

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

var noseHeadings = []int{0, 90, 180, 270}

// noseAtlas is the most detailed atlas, which records each vehicle's nose.
func noseAtlas(t *testing.T) assets.KitAtlas {
	t.Helper()
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.NotEmpty(t, atlases)
	return atlases[len(atlases)-1]
}

// trails reports whether a piece turned by turn, seen at heading while
// it moves from one point to the next, points its nose anywhere but the
// way it travels.
func trails(t *testing.T, atlas assets.KitAtlas, model string, turn, heading int, from, to city.Point) bool {
	t.Helper()
	cam := city.NewCamera()
	cam.Projection = city.Isometric
	cam.Heading = heading
	sprite, ok := atlas.Sprite(model, cam.Heading-turn)
	require.True(t, ok, "%s is not in the atlas", model)
	a, b := cam.WorldToScreen(from), cam.WorldToScreen(to)
	return sprite.Nose.X*(b.X-a.X)+sprite.Nose.Y*(b.Y-a.Y) <= 0
}

// turning reports whether a vehicle is on a corner or turning back: it
// faces the leg it was on while its next step is already along the next.
func turning(before, at, next city.Point) bool {
	came, goes := at.Sub(before), next.Sub(at)
	return came.X*goes.X+came.Y*goes.Y <= 0.9*math.Hypot(came.X, came.Y)*math.Hypot(goes.X, goes.Y)
}

// Item 151: every vehicle that drives the roads or sails the river is
// turned by vehicleTurn, and runs nose first whichever way it goes and
// whichever side the city is seen from.
func TestVehicleRunsNoseFirst(t *testing.T) {
	atlas := noseAtlas(t)
	pieces := []string{
		"car-kit/sedan", "car-kit/taxi", "car-kit/hatchback-sports",
		kitVan, kitDelivery, kitInspector, kitTruck,
		kitSkipTruck, kitSkipTruckLoaded, kitMixer,
		kitTugIn, kitTugOut,
	}
	ways := map[string]city.Point{"+x": {X: 1}, "-x": {X: -1}, "+y": {Y: 1}, "-y": {Y: -1}}
	for _, piece := range pieces {
		for name, d := range ways {
			t.Run(fmt.Sprintf("when %s drives along %s", piece, name), func(t *testing.T) {
				var trailing []int
				for _, heading := range noseHeadings {
					if trails(t, atlas, piece, vehicleTurn(d), heading, city.Point{}, d) {
						trailing = append(trailing, heading)
					}
				}

				t.Run("it should point its nose the way it travels at every heading", func(t *testing.T) {
					assert.Empty(t, trailing, "noseHeadings where the nose trails")
				})
			})
		}
	}
}

// Item 151: the traffic, driven round real roads there and back, runs
// nose first on every leg, along x and along y.
func TestTrafficRunsNoseFirst(t *testing.T) {
	atlas := noseAtlas(t)
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	snap := state.Snapshot{At: at, Sessions: []state.Session{
		{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: at},
		{ID: "b", CWD: "/p/b", State: state.Working, StartedAt: at},
		{ID: "c", CWD: "/p/c", State: state.Working, StartedAt: at},
	}}
	snap.Roads = []state.Road{
		{From: "/p/a", To: "/p/b", Files: 40, Sessions: 3, Active: 2},
		{From: "/p/a", To: "/p/c", Files: 40, Sessions: 3, Active: 2},
		{From: "/p/b", To: "/p/c", Files: 40, Sessions: 3, Active: 2},
	}
	c := city.Build(snap, city.NewLayout())
	require.NotEmpty(t, c.Streets)
	longest := 0.0
	for _, s := range c.Streets {
		longest = math.Max(longest, pathLength(s.Path))
	}
	g := &Game{}
	axes := map[string]bool{}
	trailing := map[string]bool{}
	const dt = 0.01
	for seconds := 0.0; seconds < 2*longest/carSpeed; seconds += city.Tile / carSpeed / 4 {
		before, now, next := g.cars(c, seconds-dt), g.cars(c, seconds), g.cars(c, seconds+dt)
		require.Equal(t, len(now), len(next))
		for i, car := range now {
			move := next[i].at.Sub(car.at)
			if turning(before[i].at, car.at, next[i].at) {
				continue
			}
			if math.Abs(move.X) > math.Abs(move.Y) {
				axes["x"] = true
			} else {
				axes["y"] = true
			}
			for _, heading := range noseHeadings {
				if trails(t, atlas, car.model, car.turn, heading, car.at, next[i].at) {
					trailing[fmt.Sprintf("%s turn %d at heading %d", car.model, car.turn, heading)] = true
				}
			}
		}
	}
	require.Equal(t, map[string]bool{"x": true, "y": true}, axes, "the lap has to drive along both axes")

	t.Run("when the traffic drives a full lap there and back, it should point every car's nose the way it travels at every heading", func(t *testing.T) {
		assert.Empty(t, trailing)
	})
}

// Item 151: the inspector's car drives in along +x to park at the kerb
// and drives back out the way it came, nose first both ways.
func TestInspectorRunsNoseFirst(t *testing.T) {
	atlas := noseAtlas(t)
	cases := map[string]struct {
		in   city.Inspector
		move city.Point
	}{
		"arriving": {city.Inspector{Parked: 0.5}, city.Point{X: 1}},
		"leaving":  {city.Inspector{Parked: 0.5, Leaving: true}, city.Point{X: -1}},
	}
	for name, c := range cases {
		t.Run("when the inspector's car is "+name, func(t *testing.T) {
			var trailing []int
			for _, heading := range noseHeadings {
				if trails(t, atlas, kitInspector, inspectorTurn(c.in), heading, city.Point{}, c.move) {
					trailing = append(trailing, heading)
				}
			}

			t.Run("it should point its nose the way it moves at every heading", func(t *testing.T) {
				assert.Empty(t, trailing, "noseHeadings where the nose trails")
			})
		})
	}
}
