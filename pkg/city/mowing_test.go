package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86: an overgrown lot is mowed when its session wakes.
func TestMowing(t *testing.T) {
	quiet := func(id string, days int) state.Session {
		s := session(id, cinders, state.Resting)
		s.LastActivity = now.Add(-time.Duration(days) * 24 * time.Hour)
		return s
	}
	woken := func(id string) state.Session {
		return session(id, cinders, state.Working)
	}
	at := now
	wake := func(t *testing.T, from state.Session, reduced bool) *city.Scene {
		t.Helper()
		at = now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetReducedMotion(reduced)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(from))
		s.SetSnapshot(snapshot(woken(from.ID)))
		return s
	}

	t.Run("when an overgrown session wakes", func(t *testing.T) {
		s := wake(t, quiet("a", 40), false)

		t.Run("it should start a mowing on its lot", func(t *testing.T) {
			assert.Equal(t, 0.0, s.Mowing("a"))
		})

		t.Run("and halfway through the mowing", func(t *testing.T) {
			at = now.Add(city.MowFor / 2)

			t.Run("it should report it under way", func(t *testing.T) {
				assert.InDelta(t, 0.5, s.Mowing("a"), 1e-9)
			})

			t.Run("it should keep the scene animating", func(t *testing.T) {
				assert.True(t, s.Animating())
			})
		})

		t.Run("and after it ends", func(t *testing.T) {
			at = now.Add(city.MowFor)

			t.Run("it should report it finished", func(t *testing.T) {
				assert.Equal(t, -1.0, s.Mowing("a"))
			})
		})
	})

	t.Run("when reduced motion is on and an overgrown session wakes", func(t *testing.T) {
		s := wake(t, quiet("a", 40), true)
		at = now.Add(500 * time.Millisecond)

		t.Run("it should be a cross-fade done in under half a second", func(t *testing.T) {
			assert.Equal(t, -1.0, s.Mowing("a"))
		})
	})

	t.Run("when a resting session that is not overgrown wakes", func(t *testing.T) {
		s := wake(t, quiet("a", 1), false)

		t.Run("it should start no mowing", func(t *testing.T) {
			assert.Equal(t, -1.0, s.Mowing("a"))
		})
	})

	t.Run("when an overgrown session stays asleep", func(t *testing.T) {
		at = now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(quiet("a", 40)))
		s.SetSnapshot(snapshot(quiet("a", 40)))

		t.Run("it should start no mowing", func(t *testing.T) {
			assert.Equal(t, -1.0, s.Mowing("a"))
		})
	})
}
