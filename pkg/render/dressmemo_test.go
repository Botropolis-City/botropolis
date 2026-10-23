package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 207: an overgrown district's floor was dressed again on every
// frame, and on the real history that was most of what an unfocused
// city's frame spent. It is a pure function of the district, so it is
// kept while what it reads holds, a new snapshot's equal district
// included.
func TestDressingMemo(t *testing.T) {
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	overgrown := func(t *testing.T) *city.District {
		t.Helper()
		sessions := []state.Session{
			{ID: "a", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-40 * 24 * time.Hour)},
			{ID: "b", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-41 * 24 * time.Hour)},
		}
		c := city.Build(state.Snapshot{At: at, Sessions: sessions}, city.NewLayout())
		require.Len(t, c.Districts, 1)
		return c.Districts[0]
	}

	for _, c := range []struct {
		name   string
		change func(d *city.District)
		times  int
	}{
		{"when a new snapshot's district is equal", func(*city.District) {}, 1},
		{"when its rung changes", func(d *city.District) { d.Rung = state.RungShuttered }, 2},
		{"when a building inside it moves", func(d *city.District) {
			b := d.Buildings[0]
			b.Rect = movedBy(b.Rect, city.Tile)
		}, 2},
		{"when it moves", func(d *city.District) { d.Rect = movedBy(d.Rect, city.Tile) }, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			var m districtDresser
			m.dress(overgrown(t))
			next := overgrown(t)
			c.change(next)
			got := m.dress(next)

			t.Run("it should dress what a district dressed afresh is", func(t *testing.T) {
				assert.Equal(t, dressDistrict(next), got)
			})

			t.Run("it should dress it the times it changed", func(t *testing.T) {
				assert.Equal(t, c.times, m.dressed)
			})
		})
	}
}

func movedBy(r city.Rect, dx float64) city.Rect {
	r.Min.X += dx
	r.Max.X += dx
	return r
}
