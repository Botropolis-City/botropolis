package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: a van means a message and nothing else, and it keeps to the
// land.
func TestMessageVans(t *testing.T) {
	t.Run("the van should be the kit's van, and no other mover's model", func(t *testing.T) {
		others := append([]string{kitTruck, kitDelivery, kitInspector}, kitCars...)
		assert.Equal(t, [2]any{"car-kit/van", false}, [2]any{kitVan, contains(others, kitVan)})
	})

	t.Run("a van's path should never cross the river band", func(t *testing.T) {
		start := time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)
		snap := func(messages int) state.Snapshot {
			s := state.Snapshot{At: start, Sessions: []state.Session{
				{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start},
				{ID: "b", CWD: "/p/b", State: state.Working, StartedAt: start},
			}}
			s.Roads = []state.Road{{From: "/p/a", To: "/p/b", Messages: messages, Sessions: 2}}
			return s
		}
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return start }, time.UTC)
		s.SetSnapshot(snap(0))
		s.SetSnapshot(snap(1))
		vans := s.Messages()
		require.Len(t, vans, 1)
		from, _, width, ok := s.City().RiverBand()
		require.True(t, ok)
		var wet int
		for _, p := range vans[0].Path {
			if p.X > from.X-width/2 && p.X < from.X+width/2 {
				wet++
			}
		}
		assert.Zero(t, wet)
	})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
