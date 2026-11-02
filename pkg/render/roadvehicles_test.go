package render

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Items 89 and 255: each kind of traffic on a road is its own vehicle,
// told apart by its size and shape and not by colour alone, and it keeps
// to the land.
func TestRoadVehicles(t *testing.T) {
	cargoes := []city.Cargo{city.CargoMessage, city.CargoFiles, city.CargoVisit}
	movers := append([]string{kitDelivery, kitInspector, kitTruck, kitSkipTruck, kitSkipTruckLoaded, kitMixer}, kitCars...)

	t.Run("a message's vehicle should be the kit's van", func(t *testing.T) {
		assert.Equal(t, "car-kit/van", trafficModel(city.CargoMessage))
	})

	for _, cargo := range cargoes {
		t.Run("when it carries a "+cargo.Kind().Name()+"'s cargo", func(t *testing.T) {
			t.Run("it should be no other mover's model, and no other cargo's", func(t *testing.T) {
				var others []string
				for _, o := range cargoes {
					if o != cargo {
						others = append(others, trafficModel(o))
					}
				}
				assert.False(t, slices.Contains(append(others, movers...), trafficModel(cargo)))
			})
		})
	}

	feet, err := assets.Footprints()
	require.NoError(t, err)
	length := func(model string) float64 { return feet[model].Length * trafficShrink(model) }
	others := func(model string) []string {
		var out []string
		for _, m := range append([]string{kitVan, kitDelivery, kitInspector, kitTruck}, kitCars...) {
			if m != model {
				out = append(out, m)
			}
		}
		return out
	}

	t.Run("when it is a box truck", func(t *testing.T) {
		truck := trafficModel(city.CargoFiles)
		require.Positive(t, feet[truck].Length, "no footprint for %s: the check below passes on nothing", truck)

		t.Run("it should be at least a sixth longer than any car, van or truck on the roads", func(t *testing.T) {
			var longest float64
			for _, m := range others(truck) {
				longest = max(longest, length(m))
			}
			assert.GreaterOrEqual(t, length(truck), longest*7/6)
		})
	})

	t.Run("when it is a motorbike", func(t *testing.T) {
		bike := trafficModel(city.CargoVisit)
		require.Positive(t, feet[bike].Length, "no footprint for %s: the check below passes on nothing", bike)

		t.Run("it should be no more than two thirds the length of any car, van or truck on the roads", func(t *testing.T) {
			shortest := 1e9
			for _, m := range others(bike) {
				shortest = min(shortest, length(m))
			}
			assert.LessOrEqual(t, length(bike), shortest*2/3)
		})

		t.Run("it should be no more than half the width of a car", func(t *testing.T) {
			assert.LessOrEqual(t, feet[bike].Width*trafficShrink(bike), feet[kitCars[0]].Width/2)
		})
	})

	t.Run("when a message passes between two stations on a group's line", func(t *testing.T) {
		m := city.Traffic{Path: []city.Point{{}, {X: 10}}, Since: time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)}
		at, _ := trafficPlace(m, m.Since.Add(m.For()/2), nil)

		t.Run("it should keep its lane on the road, off the centre line a carriage ran", func(t *testing.T) {
			assert.InDelta(t, laneOffset, sideOf(city.Point{}, city.Point{X: 1}, at), 1e-6)
		})
	})

	t.Run("a van's path should never cross the river band", func(t *testing.T) {
		start := time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)
		snap := func(messages int) state.Snapshot {
			s := state.Snapshot{At: start, Sessions: []state.Session{
				{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start},
				{ID: "b", CWD: "/p/b", State: state.Working, StartedAt: start},
			}}
			s.Roads = []state.Road{{From: "/p/a", To: "/p/b", Messages: messages, Sessions: 2}}
			return s
		}
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return start }, time.UTC)
		s.SetSnapshot(snap(0))
		s.SetSnapshot(snap(1))
		vans := s.Traffic()
		require.Len(t, vans, 1)
		from, _, width, ok := s.City().RiverBand()
		require.True(t, ok)
		var wet int
		for _, p := range vans[0].Path {
			if p.X > from.X-width/2 && p.X < from.X+width/2 {
				wet++
			}
		}
		assert.Zero(t, wet)
	})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
