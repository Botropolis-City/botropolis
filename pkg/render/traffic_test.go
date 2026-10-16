package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 85: a car is one working traveller on a connection, the way a
// drone is one subagent, and a connected road with nobody working on it
// keeps its tarmac and has no cars.
func TestTrafficCars(t *testing.T) {
	connected := func(active int) *city.City {
		at := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		snap := state.Snapshot{At: at, Sessions: []state.Session{
			{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: at},
			{ID: "b", CWD: "/p/b", State: state.Working, StartedAt: at},
		}}
		snap.Roads = []state.Road{{From: "/p/a", To: "/p/b", Files: 40, Sessions: 3, Active: active}}
		return city.Build(snap, city.NewLayout())
	}

	for _, active := range []int{1, 3} {
		t.Run(fmt.Sprintf("when %d travellers are working on a road", active), func(t *testing.T) {
			c := connected(active)
			require.Len(t, c.Streets, 1)

			t.Run(fmt.Sprintf("it should draw %d cars", active), func(t *testing.T) {
				assert.Len(t, (&Game{}).cars(c, 0), active)
			})
		})
	}

	t.Run("when a road is connected and nobody on it is working", func(t *testing.T) {
		c := connected(0)

		t.Run("it should keep its tarmac", func(t *testing.T) {
			assert.NotEmpty(t, c.StreetCells)
		})

		t.Run("it should draw no car", func(t *testing.T) {
			assert.Empty(t, (&Game{}).cars(c, 0))
		})
	})
}
