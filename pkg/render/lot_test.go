package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86: what a lot carries on each rung of the ladder. The building
// shows its rung in its windows; the ground around it ages.
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
		name                                     string
		building                                 *city.Building
		beacon, dark, boards, worn, cracks, weed bool
	}{
		{"live", building("live", state.Working, time.Minute), true, false, false, false, false, false},
		{"resting", building("resting", state.Resting, day), false, true, false, true, false, false},
		{"shuttered", building("shuttered", state.Resting, 10*day), false, true, true, true, true, false},
		{"overgrown", building("overgrown", state.Resting, 40*day), false, true, true, true, true, true},
	}
	for _, r := range rungs {
		t.Run("when a building is "+r.name, func(t *testing.T) {
			d := dressLot(r.building, -1)

			t.Run("it should light its beacon and sign only if live", func(t *testing.T) {
				assert.Equal(t, r.beacon, d.Beacon && d.Sign)
			})

			t.Run("it should darken its glass only once stopped", func(t *testing.T) {
				assert.Equal(t, r.dark, d.Dark)
			})

			t.Run("it should board its windows only once shuttered", func(t *testing.T) {
				assert.Equal(t, r.boards, d.Boards)
			})

			t.Run("it should wear its paving only once stopped", func(t *testing.T) {
				assert.Equal(t, r.worn, d.Wear > 0)
			})

			t.Run("it should crack its paving only once shuttered", func(t *testing.T) {
				assert.Equal(t, r.cracks, len(d.Cracks) > 0)
			})

			t.Run("it should grow weeds only once overgrown", func(t *testing.T) {
				assert.Equal(t, r.weed, len(d.Weeds) > 0)
			})
		})
	}

	t.Run("when the rungs are compared", func(t *testing.T) {
		t.Run("it should wear the paving more the longer it has been quiet", func(t *testing.T) {
			assert.Less(t, dressLot(rungs[1].building, -1).Wear, dressLot(rungs[2].building, -1).Wear)
			assert.Less(t, dressLot(rungs[2].building, -1).Wear, dressLot(rungs[3].building, -1).Wear)
		})
	})

	t.Run("when an overgrown lot is drawn", func(t *testing.T) {
		b := building("overgrown", state.Resting, 40*day)
		d := dressLot(b, -1)
		yard := lotRect(b)
		footprint := shrink(b.Rect, 0.1)

		t.Run("it should grow the same weeds in the same places every time", func(t *testing.T) {
			assert.Equal(t, d.Weeds, dressLot(b, -1).Weeds)
		})

		t.Run("it should keep every weed on its own lot", func(t *testing.T) {
			for _, w := range d.Weeds {
				assert.True(t, yard.Contains(w.At), w.At)
			}
		})

		t.Run("it should grow nothing on the building's footprint, only at its base and in the yard", func(t *testing.T) {
			for _, w := range d.Weeds {
				assert.False(t, footprint.Contains(w.At), w.At)
			}
		})

		t.Run("it should grow along the whole length of both front edges", func(t *testing.T) {
			r := b.Rect
			var east, south [5]int
			for _, w := range d.Weeds {
				switch {
				case w.At.X > r.Max.X && w.At.Y >= r.Min.Y && w.At.Y <= r.Max.Y:
					east[min(4, int(5*(w.At.Y-r.Min.Y)/r.Height()))]++
				case w.At.Y > r.Max.Y && w.At.X >= r.Min.X && w.At.X <= r.Max.X:
					south[min(4, int(5*(w.At.X-r.Min.X)/r.Width()))]++
				}
			}
			for i := range 5 {
				assert.Positive(t, east[i], "east fifth %d", i)
				assert.Positive(t, south[i], "south fifth %d", i)
			}
		})

		t.Run("it should reach further out in front than behind", func(t *testing.T) {
			r := b.Rect
			var front, back []float64
			for _, w := range d.Weeds {
				switch {
				case w.At.X > r.Max.X:
					front = append(front, w.At.X-r.Max.X)
				case w.At.Y > r.Max.Y:
					front = append(front, w.At.Y-r.Max.Y)
				case w.At.X < r.Min.X:
					back = append(back, r.Min.X-w.At.X)
				case w.At.Y < r.Min.Y:
					back = append(back, r.Min.Y-w.At.Y)
				}
			}
			assert.Greater(t, mean(front), mean(back))
		})

		t.Run("it should grow some of it in the cracks", func(t *testing.T) {
			inCracks := 0
			for _, w := range d.Weeds {
				for _, c := range d.Cracks {
					for _, p := range c {
						if p == w.At {
							inCracks++
						}
					}
				}
			}
			assert.Positive(t, inCracks)
		})

		t.Run("it should keep every crack on the lot", func(t *testing.T) {
			for _, c := range d.Cracks {
				for _, p := range c {
					assert.True(t, yard.Contains(p), p)
				}
			}
		})

		t.Run("it should use only the overgrowth pieces", func(t *testing.T) {
			for _, w := range d.Weeds {
				assert.Contains(t, overgrowthPieces, w.Piece)
			}
		})
	})

	t.Run("when two overgrown lots are drawn", func(t *testing.T) {
		a, b := building("a", state.Resting, 40*day), building("b", state.Resting, 40*day)

		t.Run("it should scatter them differently", func(t *testing.T) {
			assert.NotEqual(t, offsets(a, dressLot(a, -1).Weeds), offsets(b, dressLot(b, -1).Weeds))
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
