package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86: a district takes the freshest rung among its sessions, so its
// whole lot ages only when every session on it has.
func TestDistrictRung(t *testing.T) {
	day := 24 * time.Hour
	quiet := func(id string, for_ time.Duration) state.Session {
		s := session(id, cinders, state.Resting)
		s.LastActivity = now.Add(-for_)
		return s
	}
	district := func(t *testing.T, sessions ...state.Session) *city.District {
		t.Helper()
		c := build(t, city.NewLayout(), sessions...)
		for _, d := range c.Districts {
			if d.Root == cinders {
				return d
			}
		}
		require.FailNow(t, "no cinders district")
		return nil
	}

	for _, r := range []struct {
		rung state.Rung
		for_ time.Duration
	}{
		{state.RungResting, day},
		{state.RungShuttered, 10 * day},
		{state.RungOvergrown, 40 * day},
	} {
		t.Run("when every session in a district is "+string(r.rung), func(t *testing.T) {
			d := district(t, quiet("a", r.for_), quiet("b", r.for_+day))

			t.Run("the district's rung should be "+string(r.rung), func(t *testing.T) {
				assert.Equal(t, r.rung, d.Rung)
			})

			t.Run("and one session in it is live", func(t *testing.T) {
				d := district(t, quiet("a", r.for_), session("c", cinders, state.Working))

				t.Run("the district's rung should be live", func(t *testing.T) {
					assert.Equal(t, state.RungLive, d.Rung)
				})
			})
		})
	}

	t.Run("when its sessions are on different rungs", func(t *testing.T) {
		d := district(t, quiet("a", day), quiet("b", 40*day))

		t.Run("the district's rung should be the freshest", func(t *testing.T) {
			assert.Equal(t, state.RungResting, d.Rung)
		})
	})

	t.Run("when a district's sessions are all packed up", func(t *testing.T) {
		crate := quiet("a", 40*day)
		crate.Packed = true
		c := build(t, city.NewLayout(), crate, session("z", botropolis, state.Working))

		t.Run("there should be no district on the map", func(t *testing.T) {
			for _, d := range c.Districts {
				assert.NotEqual(t, cinders, d.Root)
			}
		})
	})
}
