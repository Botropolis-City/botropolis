package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestFirstSightings(t *testing.T) {
	sighting := func(layout *city.Layout) (*city.Scene, *time.Time) {
		clock := now
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{session("a", cinders, state.Working)}})
		return s, &clock
	}

	t.Run("when a kind is visible for the first time", func(t *testing.T) {
		s, clock := sighting(city.NewLayout())
		s.Sight([]city.Kind{city.KindDrone})

		t.Run("it should be offered once", func(t *testing.T) {
			assert.Equal(t, []city.Kind{city.KindDrone}, s.Sightings())
		})

		t.Run("when 6 s have passed", func(t *testing.T) {
			*clock = now.Add(city.SightingFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, s.Sightings())
			})
		})
	})

	t.Run("when several kinds are new at once", func(t *testing.T) {
		s, clock := sighting(city.NewLayout())
		s.Sight([]city.Kind{city.KindDrone, city.KindFlag})
		first := s.Sightings()
		*clock = now.Add(city.SightingFor)
		between := s.Sightings()
		*clock = now.Add(city.SightingFor + city.SightingGap)
		second := s.Sightings()

		t.Run("it should caption one, then the next after the gap", func(t *testing.T) {
			assert.Equal(t, [][]city.Kind{{city.KindDrone}, nil, {city.KindFlag}}, [][]city.Kind{first, between, second})
		})
	})

	t.Run("when it is visible again", func(t *testing.T) {
		s, clock := sighting(city.NewLayout())
		s.Sight([]city.Kind{city.KindDrone})
		*clock = now.Add(time.Minute)
		s.Sight([]city.Kind{city.KindDrone})

		t.Run("it should not be offered", func(t *testing.T) {
			assert.Empty(t, s.Sightings())
		})
	})

	t.Run("when every kind is marked sighted, and then every kind is visible", func(t *testing.T) {
		s, _ := sighting(city.NewLayout())
		s.MarkSighted(city.Kinds)
		s.Sight(city.Kinds)

		t.Run("it should caption none", func(t *testing.T) {
			assert.Empty(t, s.Sightings())
		})
	})

	t.Run("when Botropolis restarts", func(t *testing.T) {
		path := t.TempDir() + "/layout.json"
		layout := city.NewLayout()
		s, _ := sighting(layout)
		s.Sight([]city.Kind{city.KindDrone})
		require.NoError(t, layout.Save(path))
		loaded, err := city.LoadLayout(path)
		require.NoError(t, err)
		again, _ := sighting(loaded)
		again.Sight([]city.Kind{city.KindDrone})

		t.Run("it should remember what was seen", func(t *testing.T) {
			assert.Empty(t, again.Sightings())
		})
	})
}
