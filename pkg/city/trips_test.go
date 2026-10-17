package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// The way out of a building, which the inspector parks at the end of and
// a delivery truck sets off from.
func TestRoute(t *testing.T) {
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{session("a", cinders, state.Working)}})
	var b *city.Building
	for _, each := range s.City().Buildings() {
		if each.Session.ID == "a" {
			b = each
		}
	}
	require.NotNil(t, b)
	d := s.City().DistrictOf(b)
	route := d.Route(b)
	require.Len(t, route, 3)

	t.Run("when the way out of a building is taken", func(t *testing.T) {
		t.Run("it should start at the door", func(t *testing.T) {
			assert.Equal(t, d.Door(b), route[0])
		})

		t.Run("it should turn into the lane beside the building", func(t *testing.T) {
			assert.Greater(t, route[1].X, b.Rect.Max.X)
		})

		t.Run("it should end at the block's kerb", func(t *testing.T) {
			assert.Equal(t, d.Rect.Max.Y-city.DoorInset, route[2].Y)
		})

		t.Run("it should keep every point inside the block", func(t *testing.T) {
			for _, p := range route {
				assert.True(t, d.Rect.Contains(p), "%v is outside %v", p, d.Rect)
			}
		})
	})
}
