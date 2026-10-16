package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func still(t *testing.T, sessions ...state.Session) *city.Scene {
	t.Helper()
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	s.SetSnapshot(snapshot(sessions...))
	return s
}

func TestAnimating(t *testing.T) {
	t.Run("when a session is mid-turn", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Working))

		t.Run("it should be animating, because its worker is at the door", func(t *testing.T) {
			assert.True(t, s.Animating())
		})
	})

	t.Run("when a session wants attention", func(t *testing.T) {
		s := still(t, session("a", cinders, state.NeedsYou))

		t.Run("it should be animating, because the beacon pulses", func(t *testing.T) {
			assert.True(t, s.Animating())
		})
	})

	t.Run("when every session is resting", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Resting), session("b", botropolis, state.Resting))

		t.Run("it should be still", func(t *testing.T) {
			assert.False(t, s.Animating())
		})
	})

	t.Run("when a session is waiting on its own watch with nothing in flight", func(t *testing.T) {
		quiet := session("a", cinders, state.Waiting)
		quiet.Subagents, quiet.SubagentsInFlight = 0, 0
		s := still(t, quiet)
		require.NotEmpty(t, s.City().Buildings())

		t.Run("it should be still: nothing about it moves", func(t *testing.T) {
			assert.False(t, s.Animating())
		})
	})

	t.Run("when a waiting session still has a subagent out", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Waiting))

		t.Run("it should be animating, because a drone is circling its roof", func(t *testing.T) {
			assert.True(t, s.Animating())
		})
	})

	t.Run("when a courier is crossing a city where nothing else moves", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Resting), session("b", botropolis, state.Resting))
		require.False(t, s.Animating(), "the scene has to be still before the courier for this to mean anything")
		s.SendCourier()

		t.Run("it should be animating: the boat is the fastest thing on the map", func(t *testing.T) {
			assert.True(t, s.Animating())
		})
	})

	t.Run("when the courier has finished its run", func(t *testing.T) {
		clock := now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(session("a", cinders, state.Resting)))
		s.SendCourier()
		clock = clock.Add(city.CourierFor + time.Second)

		t.Run("it should be still again", func(t *testing.T) {
			assert.False(t, s.Animating())
		})
	})

	t.Run("when motion is reduced", func(t *testing.T) {
		s := still(t, session("a", cinders, state.Working))
		s.SetReducedMotion(true)

		t.Run("it should be still whatever the map holds", func(t *testing.T) {
			assert.False(t, s.Animating())
		})
	})

	t.Run("when the city is empty", func(t *testing.T) {
		s := still(t)

		t.Run("it should be still", func(t *testing.T) {
			assert.False(t, s.Animating())
		})
	})
}
