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

// Item 134: a message's carriage on a group's line keeps the size it had
// before the train grew, 0.9 of a wagon cut at 0.25, because it rides
// over lots, badges and station dots, and a loop wagon's size would
// cover them (the manager, for Aria, 2026-10-19).
func TestMessageCarriage(t *testing.T) {
	feet, err := assets.Footprints()
	require.NoError(t, err)
	model := kitWagons[0]
	require.Positive(t, feet[model].Length, "no wagon footprint: the check below passes on nothing")

	t.Run("when a message rides a group's line", func(t *testing.T) {
		t.Run("it should draw its carriage 0.61 cells long, the size it was before the train grew", func(t *testing.T) {
			assert.InDelta(t, 2.7*0.25*0.9, feet[model].Length*messageShrink(model), 0.01)
		})
	})
}

// Item 147: a train's engine says its model's context window, one engine
// per class as Aria picked them on 2026-10-19, and the wagons still count
// the spend.
func TestEngineByWindow(t *testing.T) {
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	snap := state.Snapshot{At: start, Sessions: []state.Session{{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start}}}
	run := func(train *city.Train) []carriage {
		c := city.Build(snap, city.NewLayout())
		c.Trains = []*city.Train{train}
		return (&Game{}).carriages(c, 0)
	}
	cases := []struct {
		name   string
		tokens int64
		from   city.WindowSource
		engine string
	}{
		{"up to 200k", 200_000, city.FromSessions, "train-kit/train-diesel-c"},
		{"up to 1M", 1_000_000, city.FromSessions, "train-kit/train-electric-bullet-a"},
		{"over 1M", 2_000_000, city.FromModelTable, "train-kit/train-electric-double-a"},
		{"unknown", 0, city.WindowUnknown, "train-kit/train-locomotive-a"},
	}
	for _, c := range cases {
		t.Run("when the model's window is "+c.name, func(t *testing.T) {
			cars := run(&city.Train{Model: "m", Wagons: 3, WindowTokens: c.tokens, WindowFrom: c.from})
			require.NotEmpty(t, cars)

			t.Run("it should run behind "+c.engine, func(t *testing.T) {
				assert.Equal(t, c.engine, cars[0].model)
			})

			t.Run("it should carry its three wagons behind the engine", func(t *testing.T) {
				assert.Len(t, cars, 4)
			})
		})
	}
}

// Item 147: every engine runs nose first, all the way round the loop and
// whichever side the city is seen from. The pipeline records where each
// train piece's front points in its sprite; the map picks the sprite from
// the direction of travel, so the two have to agree.
func TestEngineRunsNoseFirst(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.NotEmpty(t, atlases)
	atlas := atlases[len(atlases)-1]
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	c := city.Build(state.Snapshot{At: start, Sessions: []state.Session{{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start}}}, city.NewLayout())
	loop := pathLength(c.Rails)
	require.Positive(t, loop)
	cases := map[city.EngineClass]*city.Train{
		city.UpTo200k:      {Model: "m", Wagons: 1, WindowTokens: 200_000, WindowFrom: city.FromSessions},
		city.UpTo1M:        {Model: "m", Wagons: 1, WindowTokens: 1_000_000, WindowFrom: city.FromSessions},
		city.Over1M:        {Model: "m", Wagons: 1, WindowTokens: 2_000_000, WindowFrom: city.FromSessions},
		city.DefaultEngine: {Model: "m", Wagons: 1},
	}
	for class, train := range cases {
		c.Trains = []*city.Train{train}
		engine := kitEngines[class]
		seen := map[int]bool{}
		checked := 0
		for step := 0.0; step < loop/trainSpeed; step += city.Tile / trainSpeed {
			head := (&Game{}).carriages(c, step)[0]
			ahead, _ := pointAlong(c.Rails, math.Mod(head.along+city.Tile*0.1, loop))
			behind, _ := pointAlong(c.Rails, math.Mod(head.along-city.Tile*0.1+loop, loop))
			if turning(behind, head.at, ahead) {
				continue
			}
			checked++
			for _, heading := range []int{0, 90, 180, 270} {
				cam := city.NewCamera()
				cam.Projection = city.Isometric
				cam.Heading = heading
				sprite, ok := atlas.Sprite(head.model, cam.Heading-head.turn)
				require.True(t, ok, "%s is not in the atlas", head.model)
				from, to := cam.WorldToScreen(head.at), cam.WorldToScreen(ahead)
				if sprite.Nose.X*(to.X-from.X)+sprite.Nose.Y*(to.Y-from.Y) <= 0 {
					seen[heading*1000+head.turn] = true
				}
			}
		}

		require.Greater(t, checked, 4, "the loop has to be checked off its corners")

		t.Run(fmt.Sprintf("when %s runs round the loop, it should point its nose the way it travels at every turn and heading", engine), func(t *testing.T) {
			assert.Empty(t, seen, "heading*1000+turn where the nose trails")
		})
	}
}
