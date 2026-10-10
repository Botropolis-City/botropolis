package city_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 56. Aria: "Still don't see the refresh boat (to be clear the
// usage boats are there just not the refresh boat when I press u)."
//
// The boat she missed was the probe's tug, which existed only because
// `claude -p` is a real session — the same accident that dragged the
// map around and had to go in bug 54. The courier is the event given a
// boat on purpose instead.

func courierScene(t *testing.T, clock *time.Time) *city.Scene {
	t.Helper()
	s := city.NewScene(city.NewLayout())
	s.Resize(800, 600)
	s.SetClock(func() time.Time { return *clock }, time.UTC)
	s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
	return s
}

func TestCourierRunsTheWholeRiver(t *testing.T) {
	clock := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	s := courierScene(t, &clock)
	head, mouth, _, ok := s.City().RiverBand()
	require.True(t, ok)
	s.SendCourier()

	t.Run("when a refresh has just gone out", func(t *testing.T) {
		require.Len(t, s.Couriers(), 1)

		t.Run("it should set out from the river's head", func(t *testing.T) {
			assert.InDelta(t, head.Y, s.Couriers()[0].At(clock).Y, 0.001)
		})
	})

	t.Run("when it is halfway", func(t *testing.T) {
		clock = clock.Add(city.CourierFor / 2)

		t.Run("it should be midstream", func(t *testing.T) {
			assert.InDelta(t, (head.Y+mouth.Y)/2, s.Couriers()[0].At(clock).Y, 1)
		})
	})

	// Aria: "Even if the command completes, we should still render the
	// boat all the way across." The probe returns in about a second and
	// the run takes five, so a boat tied to the subprocess would vanish
	// mid-river — the despawn she already rejected once.
	t.Run("when the run is not yet done", func(t *testing.T) {
		clock = clock.Add(city.CourierFor/2 - time.Millisecond)

		t.Run("it should still be sailing, whatever the probe did", func(t *testing.T) {
			assert.Len(t, s.Couriers(), 1)
		})
	})

	t.Run("when it has run its length", func(t *testing.T) {
		clock = clock.Add(2 * time.Millisecond)

		t.Run("it should have left the map rather than sat there", func(t *testing.T) {
			assert.Empty(t, s.Couriers())
		})
	})
}

// The guard that the courier cannot bring back bug 54. It is not a
// session, so nothing about it may reach the plan — the moment it does,
// a refresh moves the river again and every boat in flight with it.
func TestCourierIsNotASession(t *testing.T) {
	clock := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	s := courierScene(t, &clock)
	before, beforeMouth, beforeWidth, _ := s.City().RiverBand()
	buildings := len(s.City().Buildings())

	s.SendCourier()
	after, afterMouth, afterWidth, _ := s.City().RiverBand()

	t.Run("when a refresh boat is launched", func(t *testing.T) {
		t.Run("it should leave the river exactly where it was", func(t *testing.T) {
			assert.Equal(t, [3]float64{before.X, beforeMouth.Y, beforeWidth},
				[3]float64{after.X, afterMouth.Y, afterWidth})
		})

		t.Run("it should raise no building", func(t *testing.T) {
			assert.Len(t, s.City().Buildings(), buildings)
		})

		t.Run("it should launch no tug, because it is not a session", func(t *testing.T) {
			assert.Empty(t, s.Voyages())
		})
	})
}

func TestCourierFollowsTheRiverIfItMoves(t *testing.T) {
	clock := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	s := courierScene(t, &clock)
	s.SendCourier()

	t.Run("when the map re-plans mid-run", func(t *testing.T) {
		clock = clock.Add(city.CourierFor / 2)
		s.SetSnapshot(snapshot(
			session("a", cinders, state.Working),
			session("b", cinders, state.Working),
			session("c", cinders, state.Working)))
		require.Len(t, s.Couriers(), 1)

		t.Run("it should still be on the water", func(t *testing.T) {
			// Written with the rule rather than retrofitted: bug 54a
			// was a boat sailing a river that had moved under it.
			assert.Zero(t, aground(s.City(), s.Couriers()[0].At(clock)))
		})
	})
}

func TestCourierCardTeachesTheKey(t *testing.T) {
	t.Run("when a courier is pointed at", func(t *testing.T) {
		card := (&city.Courier{}).Card()

		// Aria had not realised the refresh existed at all, so the
		// first time she points at one it has to say what the key does.
		t.Run("it should name the key that sent it", func(t *testing.T) {
			assert.Contains(t, card.Lines[0], city.RefreshKey)
		})
	})
}
