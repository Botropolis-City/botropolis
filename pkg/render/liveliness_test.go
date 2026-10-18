package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: no level of liveliness loses what the movers say. Every mover
// on the map at lively is on it at every level, and at still each one
// stands where it is, as a marker.
func TestMoversAtEveryLevel(t *testing.T) {
	start := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	snap := state.Snapshot{At: start, Sessions: []state.Session{
		{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start, SubagentsInFlight: 2, APIErrors: 2},
		{ID: "b", CWD: "/p/b", State: state.Working, StartedAt: start},
	}}
	snap.Roads = []state.Road{{From: "/p/a", To: "/p/b", Files: 40, Sessions: 3, Active: 2}}
	c := city.Build(snap, city.NewLayout())
	c.Rails = []city.Point{{X: 0, Y: 0}, {X: 400, Y: 0}, {X: 400, Y: 400}, {X: 0, Y: 0}}
	c.Trains = []*city.Train{{Model: "m", Wagons: 2}}
	require.Len(t, c.Streets, 1)
	var b *city.Building
	for _, each := range c.Buildings() {
		if each.Session.ID == "a" {
			b = each
		}
	}
	require.NotNil(t, b)
	roof := city.RectAt(0, 0, 40, 60)
	cam := city.NewCamera()
	dronesAt := func(seconds float64) []city.Point {
		var out []city.Point
		for seat := 0; seat < b.Cranes; seat++ {
			at, _ := droneAt(cam, city.Flight{Seat: seat, Over: b.Rect.Center(), Climb: 1, Fade: 1}, 0, seconds)
			out = append(out, at)
		}
		return out
	}

	working := func(b *city.Building) bool {
		return city.NewScene(city.NewLayout()).Scaffold(b) > 0
	}

	// frame is what one frame draws, a second after another.
	type frame struct {
		cars      []car
		carriages []carriage
		drones    []city.Point
		smoke     []puff
		working   bool
	}
	at := func(level city.Liveliness, after time.Duration) frame {
		m := newMotionClock(start)
		m.step(start.Add(after), level)
		g := &Game{}
		return frame{
			cars:      g.cars(c, m.ambient),
			carriages: g.carriages(c, m.ambient),
			drones:    dronesAt(m.ambient),
			smoke:     smokePuffs(roof, 1, b.Smoke, m.ambient),
			working:   working(b),
		}
	}

	for _, level := range city.LivelinessLevels {
		t.Run("when the level is "+level.String(), func(t *testing.T) {
			f := at(level, time.Second)

			t.Run("it should draw two cars", func(t *testing.T) {
				assert.Len(t, f.cars, 2)
			})

			t.Run("it should draw two drones", func(t *testing.T) {
				assert.Len(t, f.drones, 2)
			})

			t.Run("it should draw the train's carriages", func(t *testing.T) {
				assert.Len(t, f.carriages, 3)
			})

			t.Run("it should scaffold the roof", func(t *testing.T) {
				assert.True(t, f.working)
			})

			t.Run("it should draw the smoke", func(t *testing.T) {
				assert.Len(t, f.smoke, 2)
			})
		})
	}

	t.Run("when the level is still", func(t *testing.T) {
		first, second := at(city.Still, time.Second), at(city.Still, 2*time.Second)

		t.Run("it should draw each mover at the same place in two frames a second apart", func(t *testing.T) {
			assert.Equal(t, first, second)
		})
	})

	t.Run("when the level is lively", func(t *testing.T) {
		first, second := at(city.Lively, time.Second), at(city.Lively, 2*time.Second)

		t.Run("it should draw the movers somewhere else a second later", func(t *testing.T) {
			assert.NotEqual(t, first, second)
		})
	})
}
