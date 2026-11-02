package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 333: sessions packed up in one move leave as a convoy, one truck
// after another, the whole convoy gone within a bound.
func TestConvoy(t *testing.T) {
	packAll := func(t *testing.T, n int, reduced bool) []city.Haul {
		t.Helper()
		var lots, crates []state.Session
		for i := range n {
			s := session(fmt.Sprintf("s%03d", i), fmt.Sprintf("/w/p%03d", i), state.Resting)
			lots = append(lots, s)
			s.Packed = true
			crates = append(crates, s)
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetReducedMotion(reduced)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(lots...))
		s.SetSnapshot(snapshot(crates...))
		hauls := s.Hauls()
		require.Len(t, hauls, n)
		return hauls
	}
	starts := func(hauls []city.Haul) []time.Time {
		var out []time.Time
		for _, h := range hauls {
			out = append(out, h.Since)
		}
		return out
	}

	for _, n := range []int{5, 200} {
		t.Run(fmt.Sprintf("when %d sessions are packed up in one move", n), func(t *testing.T) {
			hauls := packAll(t, n, false)
			at := starts(hauls)

			t.Run("it should start the first truck at once", func(t *testing.T) {
				assert.Equal(t, now, at[0])
			})

			t.Run("it should send each truck after the one before it", func(t *testing.T) {
				for i := 1; i < len(at); i++ {
					require.True(t, at[i].After(at[i-1]), "truck %d leaves at %v, not after %v", i, at[i], at[i-1])
				}
			})

			t.Run("it should send the last truck within the convoy's bound", func(t *testing.T) {
				assert.LessOrEqual(t, at[len(at)-1].Sub(now), city.ConvoyMax)
			})
		})
	}

	t.Run("when sessions are packed up in one move under reduced motion", func(t *testing.T) {
		hauls := packAll(t, 5, true)

		t.Run("it should cross-fade them all at once", func(t *testing.T) {
			for _, at := range starts(hauls) {
				require.Equal(t, now, at)
			}
		})
	})
}
