package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86: a session resting and waking is drawn as it happens.
func TestSettling(t *testing.T) {
	live := session("a", cinders, state.Working)
	stopped := session("a", cinders, state.Resting)
	quiet := live
	quiet.Subagents, quiet.SubagentsInFlight, quiet.State = 0, 0, state.NeedsYou
	at := now
	change := func(t *testing.T, from, to state.Session, reduced bool) *city.Scene {
		t.Helper()
		at = now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetReducedMotion(reduced)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(from))
		s.SetSnapshot(snapshot(to))
		return s
	}

	t.Run("when a live session stops", func(t *testing.T) {
		s := change(t, live, stopped, false)
		kind, p := s.Settling("a")

		t.Run("it should start resting", func(t *testing.T) {
			assert.Equal(t, city.SettleRest, kind)
		})

		t.Run("it should start at the beginning", func(t *testing.T) {
			assert.Equal(t, 0.0, p)
		})

		t.Run("and the clock is advanced by half of RestFor", func(t *testing.T) {
			at = now.Add(city.RestFor / 2)
			_, p := s.Settling("a")

			t.Run("it should be halfway", func(t *testing.T) {
				assert.InDelta(t, 0.5, p, 1e-9)
			})

			t.Run("it should keep the scene animating", func(t *testing.T) {
				assert.True(t, s.Animating())
			})
		})

		t.Run("and the clock is advanced by RestFor", func(t *testing.T) {
			at = now.Add(city.RestFor)
			kind, _ := s.Settling("a")

			t.Run("it should be finished", func(t *testing.T) {
				assert.Equal(t, city.SettleNone, kind)
			})
		})
	})

	t.Run("when a stopped session wakes", func(t *testing.T) {
		s := change(t, stopped, quiet, false)
		kind, _ := s.Settling("a")

		t.Run("it should start waking", func(t *testing.T) {
			assert.Equal(t, city.SettleWake, kind)
		})

		t.Run("and the clock is advanced by WakeFor", func(t *testing.T) {
			at = now.Add(city.WakeFor)
			kind, _ := s.Settling("a")

			t.Run("it should be finished", func(t *testing.T) {
				assert.Equal(t, city.SettleNone, kind)
			})
		})

		t.Run("it should wake quicker than it rests", func(t *testing.T) {
			assert.Less(t, city.WakeFor, city.RestFor)
		})
	})

	t.Run("when reduced motion is on", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			from, to state.Session
		}{
			{"and a live session stops", live, stopped},
			{"and a stopped session wakes", stopped, quiet},
		} {
			t.Run(c.name, func(t *testing.T) {
				s := change(t, c.from, c.to, true)
				at = now.Add(500 * time.Millisecond)
				kind, _ := s.Settling("a")

				t.Run("it should be a cross-fade done in under half a second", func(t *testing.T) {
					assert.Equal(t, city.SettleNone, kind)
				})
			})
		}
	})

	t.Run("when a session first appears already stopped", func(t *testing.T) {
		at = now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(session("b", cinders, state.Working)))
		s.SetSnapshot(snapshot(session("b", cinders, state.Working), stopped))
		kind, _ := s.Settling("a")

		t.Run("it should start no transition", func(t *testing.T) {
			assert.Equal(t, city.SettleNone, kind)
		})
	})

	t.Run("when a session's state does not change", func(t *testing.T) {
		s := change(t, stopped, stopped, false)
		kind, _ := s.Settling("a")

		t.Run("it should start no transition", func(t *testing.T) {
			require.NotNil(t, s)
			assert.Equal(t, city.SettleNone, kind)
		})
	})
}
