package render

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 153: every road vehicle, wherever its way comes from, drives the
// lane on the right of its travel, and turns back across the road
// rather than jumping lanes.
func TestEveryRoadVehicleKeepsRight(t *testing.T) {
	t0 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	run := 6 * city.CellSize
	reversed := func(p []city.Point) []city.Point {
		out := make([]city.Point, len(p))
		for i, q := range p {
			out[len(p)-1-i] = q
		}
		return out
	}

	type placed struct{ at, dir city.Point }
	vehicles := map[string]func(from, to city.Point) placed{
		"a traffic car on its way there": func(from, to city.Point) placed {
			at, dir := carPlace([]city.Point{from, to}, 0.25, nil)
			return placed{at, dir}
		},
		"a traffic car on its way back": func(from, to city.Point) placed {
			at, dir := carPlace([]city.Point{to, from}, 0.75, nil)
			return placed{at, dir}
		},
		"a message van": func(from, to city.Point) placed {
			m := city.Message{Path: []city.Point{from, to}, Since: t0}
			at, dir := messagePlace(m, t0.Add(m.For()/2), nil)
			return placed{at, dir}
		},
		"the delivery truck": func(from, to city.Point) placed {
			d := city.Delivery{Path: []city.Point{from, to}, Since: t0}
			at, dir := deliveryPlace(d, t0.Add(d.For()/2), nil)
			return placed{at, dir}
		},
		"the depot's haul truck": func(from, to city.Point) placed {
			h := city.Haul{From: from, To: to, Since: t0, For: city.PackFor}
			at, dir := haulPlace(h, t0.Add(city.PackFor/3))
			return placed{at, dir}
		},
		"a site truck on its way in": func(from, to city.Point) placed {
			in := []city.Point{from, to}
			at, dir := jobPlace(in, reversed(in), 0.5, nil)
			return placed{at, dir}
		},
		"a site truck on its way out": func(from, to city.Point) placed {
			in := []city.Point{to, from}
			at, dir := jobPlace(in, reversed(in), 1.5, nil)
			return placed{at, dir}
		},
	}
	for vehicle, place := range vehicles {
		for way, d := range laneWays {
			t.Run("when "+vehicle+" travels along "+way, func(t *testing.T) {
				from := city.Point{X: 7 * city.CellSize, Y: 11 * city.CellSize}
				to := from.Add(d.Scale(run))
				p := place(from, to)

				t.Run("it should sit half a lane right of the centre line", func(t *testing.T) {
					assert.InDelta(t, laneOffset, sideOf(from, d, p.at), 1e-6)
				})
				t.Run("it should face its travel", func(t *testing.T) {
					assert.Equal(t, vehicleTurn(d), vehicleTurn(p.dir))
				})
			})
		}
	}

	t.Run("when a site truck turns back at its stop", func(t *testing.T) {
		in := []city.Point{{X: 0, Y: 0}, {X: run, Y: 0}, {X: run, Y: run}}
		const n = 400
		longest, drove := 0.0, 0.0
		prev, _ := jobPlace(in, reversed(in), 0, nil)
		for i := 1; i <= n; i++ {
			at, _ := jobPlace(in, reversed(in), 2*float64(i)/n, nil)
			longest = math.Max(longest, math.Hypot(at.X-prev.X, at.Y-prev.Y))
			prev = at
		}
		// Along runs 0 to 1 on the way in and 1 to 2 on the way out,
		// which includes turning back, so each half has its own step.
		trip := pathLength(keepRight(append(append([]city.Point{}, in...), reversed(in)...)))
		inLen := pathLength(keepRight(in))
		drove = math.Max(inLen, trip-inLen) / (n / 2)

		t.Run("it should never move further in a step than it drove", func(t *testing.T) {
			assert.LessOrEqual(t, longest, drove+1e-9)
		})
	})

	t.Run("when the inspector's car is arriving", func(t *testing.T) {
		park := city.Point{X: 20 * city.CellSize, Y: 4 * city.CellSize}
		at, dir := inspectorPlace(city.Inspector{Park: park, Parked: 0.5})

		t.Run("it should sit half a lot lane right of the centre line", func(t *testing.T) {
			assert.InDelta(t, lotLaneOffset, sideOf(park, city.Point{X: 1}, at), 1e-6)
		})
		t.Run("it should face its travel", func(t *testing.T) {
			assert.Equal(t, vehicleTurn(city.Point{X: 1}), vehicleTurn(dir))
		})
	})

	t.Run("when the inspector's car is leaving", func(t *testing.T) {
		park := city.Point{X: 20 * city.CellSize, Y: 4 * city.CellSize}
		at, dir := inspectorPlace(city.Inspector{Park: park, Parked: 0.5, Leaving: true})
		const n = 400
		longest := 0.0
		facing := 0
		prev, pdir := inspectorPlace(city.Inspector{Park: park, Parked: 1, Leaving: true})
		for i := 1; i <= n; i++ {
			next, ndir := inspectorPlace(city.Inspector{Park: park, Parked: 1 - float64(i)/n, Leaving: true})
			move := next.Sub(prev)
			longest = math.Max(longest, math.Hypot(move.X, move.Y))
			// A step that straddles a corner faces the leg it reached.
			straddles := vehicleTurn(pdir) != vehicleTurn(ndir)
			if !straddles && math.Hypot(move.X, move.Y) > 1e-9 && vehicleTurn(move) != vehicleTurn(ndir) {
				facing++
			}
			prev, pdir = next, ndir
		}
		drove := (city.InspectorRun + 2*lotLaneOffset) / n

		t.Run("it should sit half a lot lane right of the centre line, heading out", func(t *testing.T) {
			assert.InDelta(t, lotLaneOffset, sideOf(park, city.Point{X: -1}, at), 1e-6)
		})
		t.Run("it should face its travel", func(t *testing.T) {
			assert.Equal(t, vehicleTurn(city.Point{X: -1}), vehicleTurn(dir))
		})
		t.Run("it should face its travel at every step of the way out", func(t *testing.T) {
			assert.Zero(t, facing)
		})
		t.Run("it should never move further in a step than it drove", func(t *testing.T) {
			assert.LessOrEqual(t, longest, drove+1e-9)
		})
	})

	t.Run("when the delivery is on the bridge", func(t *testing.T) {
		bridge := city.Bridge{
			Land: city.Point{X: 0.5 * city.CellSize, Y: 0.5 * city.CellSize},
			Far:  city.Point{X: 12.5 * city.CellSize, Y: 0.5 * city.CellSize},
			Ramp: []city.Cell{{Col: 1}, {Col: 2}, {Col: 3}},
		}
		d := city.Delivery{Path: []city.Point{bridge.Land, bridge.Far}, Since: t0}
		now := t0.Add(d.For() / 5)
		at, _ := deliveryPlace(d, now, nil)

		t.Run("it should keep its lane on the deck", func(t *testing.T) {
			assert.InDelta(t, laneOffset, sideOf(bridge.Land, city.Point{X: 1}, at), 1e-6)
		})
		t.Run("it should ride as high as it would on the centre line", func(t *testing.T) {
			assert.Equal(t, bridge.Rise(d.At(now)), bridge.Rise(at))
		})
	})

	t.Run("when a message rides the rails", func(t *testing.T) {
		m := city.Message{Path: []city.Point{{}, {X: run}}, Since: t0, Train: true}
		now := t0.Add(m.For() / 2)
		at, _ := messagePlace(m, now, nil)

		t.Run("it should stay on the rails' centre line", func(t *testing.T) {
			assert.Equal(t, m.At(now), at)
		})
	})
}
