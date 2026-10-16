package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86: what a lot carries on each rung of the ladder.
func TestDressLot(t *testing.T) {
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	building := func(id string, st state.State, quiet time.Duration) *city.Building {
		s := state.Session{ID: id, CWD: "/src/kiln", State: st, LastActivity: at.Add(-quiet), ContextPercent: 30}
		c := city.Build(state.Snapshot{At: at, Sessions: []state.Session{s}}, city.NewLayout())
		require.Len(t, c.Buildings(), 1)
		return c.Buildings()[0]
	}
	day := 24 * time.Hour
	rungs := []struct {
		name                       string
		building                   *city.Building
		beacon, sign, boards, weed bool
	}{
		{"live", building("live", state.Working, time.Minute), true, true, false, false},
		{"resting", building("resting", state.Resting, day), false, false, false, false},
		{"shuttered", building("shuttered", state.Resting, 10*day), false, false, true, false},
		{"overgrown", building("overgrown", state.Resting, 40*day), false, false, true, true},
	}
	for _, r := range rungs {
		t.Run("when a building is "+r.name, func(t *testing.T) {
			d := dressLot(r.building, -1)

			t.Run("it should light its beacon only if live", func(t *testing.T) {
				assert.Equal(t, r.beacon, d.Beacon)
			})

			t.Run("it should hang its sign only if live", func(t *testing.T) {
				assert.Equal(t, r.sign, d.Sign)
			})

			t.Run("it should board its windows only once shuttered", func(t *testing.T) {
				assert.Equal(t, r.boards, d.Boards)
			})

			t.Run("it should grow weeds only once overgrown", func(t *testing.T) {
				assert.Equal(t, r.weed, len(d.Weeds) > 0)
			})
		})
	}

	t.Run("when an overgrown lot is drawn twice", func(t *testing.T) {
		b := building("overgrown", state.Resting, 40*day)

		t.Run("it should grow the same weeds in the same places", func(t *testing.T) {
			assert.Equal(t, dressLot(b, -1).Weeds, dressLot(b, -1).Weeds)
		})
	})

	t.Run("when two overgrown lots are drawn", func(t *testing.T) {
		a, b := building("a", state.Resting, 40*day), building("b", state.Resting, 40*day)

		t.Run("it should scatter them differently", func(t *testing.T) {
			assert.NotEqual(t, offsets(a, dressLot(a, -1).Weeds), offsets(b, dressLot(b, -1).Weeds))
		})
	})

	t.Run("when a weed is placed", func(t *testing.T) {
		b := building("overgrown", state.Resting, 40*day)

		t.Run("it should stand on the lot", func(t *testing.T) {
			for _, w := range dressLot(b, -1).Weeds {
				assert.True(t, b.Rect.Contains(w.At), w.At)
			}
		})

		t.Run("it should be one of the overgrowth pieces", func(t *testing.T) {
			for _, w := range dressLot(b, -1).Weeds {
				assert.Contains(t, overgrowthPieces, w.Piece)
			}
		})
	})

	t.Run("when a lot is being mowed", func(t *testing.T) {
		b := building("woken", state.Working, time.Minute)
		grown := len(dressLot(building("woken", state.Resting, 40*day), -1).Weeds)

		t.Run("and the mowing has just begun", func(t *testing.T) {
			t.Run("it should still carry every weed", func(t *testing.T) {
				assert.Len(t, dressLot(b, 0).Weeds, grown)
			})
		})

		t.Run("and the mowing is halfway", func(t *testing.T) {
			t.Run("it should have cleared about half", func(t *testing.T) {
				assert.InDelta(t, grown/2, len(dressLot(b, 0.5).Weeds), 1)
			})
		})
	})
}

func offsets(b *city.Building, weeds []weed) []city.Point {
	out := make([]city.Point, len(weeds))
	for i, w := range weeds {
		out[i] = city.Point{X: w.At.X - b.Rect.Min.X, Y: w.At.Y - b.Rect.Min.Y}
	}
	return out
}

func TestBoards(t *testing.T) {
	r := city.RectAt(100, 50, 60, 80)
	planks := boards(r)

	t.Run("when a shuttered building is boarded", func(t *testing.T) {
		t.Run("it should cross each visible face with two planks", func(t *testing.T) {
			assert.Len(t, planks, 4)
		})

		t.Run("it should keep every plank on the building", func(t *testing.T) {
			for _, p := range planks {
				assert.True(t, r.Contains(p[0]) && r.Contains(p[1]), p)
			}
		})
	})
}
