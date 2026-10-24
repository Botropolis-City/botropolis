package city_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 219, Aria 2026-10-24: the bridge runs off into the haze. The deck
// runs on past where it used to end and fades out over open sea within a
// few cells, and a truck fades with it and is gone at the far end.
func TestBridgeIntoTheHaze(t *testing.T) {
	c := build(t, city.NewLayout(), session("a", cinders, state.Working), session("b", mullet, state.Resting))
	bridge := c.Bridge()
	require.NotEmpty(t, bridge.Deck, "the bridge has no deck: every check below passes on nothing")
	island := c.Island()
	out := math.Copysign(1, bridge.Far.X-bridge.Land.X)
	along := func(d float64) city.Point { return city.Point{X: bridge.Land.X + out*d, Y: bridge.Land.Y} }
	hazeAt := (bridge.Haze.X - bridge.Land.X) * out
	farAt := (bridge.Far.X - bridge.Land.X) * out

	t.Run("when the city has a bridge", func(t *testing.T) {
		t.Run("it should start the haze where the deck used to end, a cell past the island's reach", func(t *testing.T) {
			edge := island.Reach().Min.X - city.CellSize
			if out > 0 {
				edge = island.Reach().Max.X + city.CellSize
			}
			assert.Equal(t, city.Point{X: edge, Y: bridge.Land.Y}, bridge.Haze)
		})

		t.Run("it should start the haze over open sea", func(t *testing.T) {
			g, _ := island.At(bridge.Haze)
			assert.Equal(t, city.GroundSea, g)
		})

		t.Run("it should run the deck on to a far end past the haze", func(t *testing.T) {
			assert.Equal(t, [2]any{true, city.CellOf(bridge.Far)}, [2]any{farAt > hazeAt, bridge.Deck[len(bridge.Deck)-1]})
		})

		t.Run("it should be gone within HazeCells of the haze's start", func(t *testing.T) {
			assert.InDelta(t, city.HazeCells*city.CellSize, farAt-hazeAt, 1e-9)
		})

		t.Run("it should show the deck whole up to the haze", func(t *testing.T) {
			var dim []float64
			for d := 0.0; d <= hazeAt; d += city.CellSize / 8 {
				if a := bridge.Alpha(along(d)); a != 1 {
					dim = append(dim, d)
				}
			}
			assert.Empty(t, dim)
		})

		t.Run("it should fade it to nothing at the far end", func(t *testing.T) {
			assert.Equal(t, 0.0, bridge.Alpha(bridge.Far))
		})

		t.Run("it should never grow clearer further out", func(t *testing.T) {
			var brighter []float64
			last := 1.0
			for d := hazeAt; d <= farAt+city.CellSize; d += city.CellSize / 8 {
				a := bridge.Alpha(along(d))
				if a > last+1e-12 {
					brighter = append(brighter, d)
				}
				last = a
			}
			assert.Empty(t, brighter)
		})
	})

	t.Run("when a truck has merged and is driving off", func(t *testing.T) {
		start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
		clock := start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		pr := func(st string) claude.PR {
			return claude.PR{Number: 1, URL: "https://github.com/o/r/pull/1", Repository: "o/r", State: st}
		}
		with := func(st string) state.Session {
			s := session("a", cinders, state.Working)
			s.PRs = []claude.PR{pr(st)}
			return s
		}
		s.SetSnapshot(snapshot(with(claude.PROpen)))
		s.SetSnapshot(snapshot(with(claude.PRMerged)))
		trucks := s.Deliveries()
		require.Len(t, trucks, 1)
		trip := trucks[0].For()

		t.Run("and it has not yet reached the far end", func(t *testing.T) {
			clock = start.Add(trip - 10*time.Millisecond)
			left := s.Deliveries()

			t.Run("it should still be on its way, in the haze", func(t *testing.T) {
				require.Len(t, left, 1)
				b := s.City().Bridge()
				a := b.Alpha(left[0].At(clock))
				assert.True(t, a > 0 && a < 1, "alpha %v", a)
			})
		})

		t.Run("and it reaches the far end", func(t *testing.T) {
			clock = start.Add(trip)

			t.Run("it should be removed", func(t *testing.T) {
				assert.Empty(t, s.Deliveries())
			})
		})
	})

	t.Run("when the view is fitted", func(t *testing.T) {
		t.Run("it should frame what stops short of the haze", func(t *testing.T) {
			assert.False(t, c.Extent().Contains(bridge.Haze), "extent %v, haze %v", c.Extent(), bridge.Haze)
		})
	})

	t.Run("when the minimap is drawn", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", mullet, state.Resting))
		box := city.RectAt(600, 500, 160, 96)
		m := s.Minimap(box)

		t.Run("it should leave the deck's far end outside its box", func(t *testing.T) {
			assert.False(t, box.Contains(m.Project(s.City().Bridge().Far)))
		})
	})
}
