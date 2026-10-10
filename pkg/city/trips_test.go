package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// worksite is a one-session scene whose clock the test moves, and the
// building whose worker it is watching.
type worksite struct {
	scene *city.Scene
	at    time.Time
}

func newWorksite(t *testing.T) *worksite {
	t.Helper()
	w := &worksite{at: now}
	w.scene = city.NewScene(city.NewLayout())
	w.scene.SetClock(func() time.Time { return w.at }, time.UTC)
	w.scene.Resize(800, 600)
	return w
}

// calling sets the session's tool call and takes a snapshot at the
// current moment; an empty tool is a session between calls.
func (w *worksite) calling(tool string) {
	s := session("a", cinders, state.Working)
	s.Tool = tool
	w.scene.SetSnapshot(state.Snapshot{At: w.at, Sessions: []state.Session{s}})
}

func (w *worksite) tick(d time.Duration) { w.at = w.at.Add(d) }

func (w *worksite) building(t *testing.T) *city.Building {
	t.Helper()
	for _, b := range w.scene.City().Buildings() {
		if b.Session.ID == "a" {
			return b
		}
	}
	t.Fatal("the city has no building for session a")
	return nil
}

func (w *worksite) worker(t *testing.T) city.Point {
	t.Helper()
	return w.scene.Worker(w.building(t))
}

func (w *worksite) door(t *testing.T) city.Point {
	t.Helper()
	b := w.building(t)
	return w.scene.City().DistrictOf(b).Door(b)
}

func TestWorker(t *testing.T) {
	t.Run("when no tool call has ever run", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")

		t.Run("it should wait at the door", func(t *testing.T) {
			assert.Equal(t, w.door(t), w.worker(t))
		})
	})

	t.Run("when the first snapshot already has a call in flight", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("Bash")
		w.tick(city.TripFor)

		t.Run("it should stay at the door, because the city only took note", func(t *testing.T) {
			assert.Equal(t, w.door(t), w.worker(t))
		})
	})

	t.Run("when a tool call starts", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")
		w.calling("Bash")
		w.tick(city.TripFor / 2)

		t.Run("it should have left the door", func(t *testing.T) {
			assert.NotEqual(t, w.door(t), w.worker(t))
		})
	})

	t.Run("when a tool call has run longer than the trip", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")
		w.calling("Bash")
		w.tick(2 * city.TripFor)
		b := w.building(t)
		route := w.scene.City().DistrictOf(b).Route(b)

		t.Run("it should be waiting at the kerb", func(t *testing.T) {
			assert.Equal(t, route[len(route)-1], w.worker(t))
		})
	})

	t.Run("when the call ends after the worker has arrived", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")
		w.calling("Bash")
		w.tick(2 * city.TripFor)
		w.calling("")
		w.tick(2 * city.TripFor)

		t.Run("it should be back at the door", func(t *testing.T) {
			assert.Equal(t, w.door(t), w.worker(t))
		})
	})

	t.Run("when the call ends while the worker is still on its way out", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")
		w.calling("Bash")
		w.tick(city.TripFor / 4)
		turned := w.worker(t)
		w.calling("")
		w.tick(city.TripFor / 8)

		t.Run("it should turn round from where it had got to", func(t *testing.T) {
			assert.Less(t, hypotOf(w.door(t), w.worker(t)), hypotOf(w.door(t), turned))
		})
	})

	t.Run("when motion is reduced", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")
		w.calling("Bash")
		w.tick(city.TripFor / 2)
		w.scene.SetReducedMotion(true)

		t.Run("it should stand at the door and not move", func(t *testing.T) {
			assert.Equal(t, w.door(t), w.worker(t))
		})
	})
}

func TestRoute(t *testing.T) {
	w := newWorksite(t)
	w.calling("")
	b := w.building(t)
	d := w.scene.City().DistrictOf(b)
	route := d.Route(b)
	require.Len(t, route, 3)

	t.Run("when a worker leaves its building", func(t *testing.T) {
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

func hypotOf(a, b city.Point) float64 {
	d := a.Sub(b)
	return d.X*d.X + d.Y*d.Y
}

func TestWorkerTurn(t *testing.T) {
	t.Run("when the worker is on its way out", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")
		w.calling("Bash")

		t.Run("it should face down the route", func(t *testing.T) {
			assert.Equal(t, 90, w.scene.WorkerTurn(w.building(t)))
		})
	})

	t.Run("when the worker is on its way home", func(t *testing.T) {
		w := newWorksite(t)
		w.calling("")
		w.calling("Bash")
		w.tick(city.TripFor)
		w.calling("")

		t.Run("it should face back up it", func(t *testing.T) {
			assert.Equal(t, 270, w.scene.WorkerTurn(w.building(t)))
		})
	})
}
