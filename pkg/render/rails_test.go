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

// Item 134: a train's carriages are spaced by their own lengths, read
// from the atlas, so they cannot overlap whatever scale the pipeline cuts
// them at; the rails sit under the wagon's body; and the loop holds the
// longest trains with room between them.
func TestTrainOnTheLoop(t *testing.T) {
	feet, err := assets.Footprints()
	require.NoError(t, err)
	length := func(model string) float64 { return feet[model].Length * city.CellSize }
	wagon := feet[kitWagons[0]]
	require.Positive(t, wagon.Width, "no wagon footprint: the checks below pass on nothing")

	smallest := func() *city.City {
		start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
		snap := state.Snapshot{At: start, Sessions: []state.Session{{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start}}}
		return city.Build(snap, city.NewLayout())
	}

	t.Run("when a six-wagon train is placed on the loop", func(t *testing.T) {
		c := smallest()
		c.Trains = []*city.Train{{Model: "m", Wagons: city.MaxWagons}}
		cars := (&Game{}).carriages(c, 0)
		require.Len(t, cars, city.MaxWagons+1)

		t.Run("it should leave a gap between every two neighbouring carriages' footprints", func(t *testing.T) {
			for i := 1; i < len(cars); i++ {
				apart := math.Mod(cars[i-1].along-cars[i].along+cars[i].loop, cars[i].loop)
				need := (length(cars[i-1].model)+length(cars[i].model))/2 + coupling
				assert.GreaterOrEqual(t, apart+1e-6, need, "carriages %d and %d overlap", i-1, i)
			}
		})

		t.Run("it should draw the rails at least half as wide apart as the wagon is wide, and less than the wagon's width", func(t *testing.T) {
			body := wagon.Width * city.CellSize
			assert.True(t, railGauge() >= body/2 && railGauge() < body, "gauge %.1f against a body %.1f wide", railGauge(), body)
		})
	})

	t.Run(fmt.Sprintf("when %d six-wagon trains run on the smallest test city's loop", len(city.ContainerHues)), func(t *testing.T) {
		c := smallest()
		for hue := range city.ContainerHues {
			c.Trains = append(c.Trains, &city.Train{Model: fmt.Sprint("m", hue), Wagons: city.MaxWagons, Hue: hue})
		}
		cars := (&Game{}).carriages(c, 0)
		require.Len(t, cars, len(city.ContainerHues)*(city.MaxWagons+1))

		t.Run("it should leave a gap of at least one wagon length between every two trains", func(t *testing.T) {
			per := city.MaxWagons + 1
			for k := range c.Trains {
				head := cars[k*per]
				ahead := cars[((k+1)%len(c.Trains))*per+per-1]
				apart := math.Mod(ahead.along-head.along+head.loop, head.loop)
				clear := apart - (length(head.model)+length(ahead.model))/2
				assert.GreaterOrEqual(t, clear, wagon.Length*city.CellSize, "train %d's head is %.1f behind the tail of the train ahead", k, clear)
			}
		})
	})
}
