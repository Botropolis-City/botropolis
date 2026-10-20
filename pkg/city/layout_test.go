package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestSetAside(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	session := state.Session{ID: "a", State: state.NeedsYou, LastActivity: now.Add(-time.Hour)}

	t.Run("when the layout is nil", func(t *testing.T) {
		var layout *city.Layout

		t.Run("it should not set the session aside", func(t *testing.T) {
			assert.False(t, layout.SetAside(session, now))
		})
	})

	t.Run("when the session was marked done", func(t *testing.T) {
		t.Run("and it has not moved since", func(t *testing.T) {
			layout := &city.Layout{Done: map[string]time.Time{"a": session.LastActivity}}

			t.Run("it should set it aside", func(t *testing.T) {
				assert.True(t, layout.SetAside(session, now))
			})
		})

		t.Run("and it has moved since", func(t *testing.T) {
			layout := &city.Layout{Done: map[string]time.Time{"a": session.LastActivity.Add(-time.Minute)}}

			t.Run("it should not set it aside", func(t *testing.T) {
				assert.False(t, layout.SetAside(session, now))
			})
		})
	})

	t.Run("when the session is snoozed", func(t *testing.T) {
		t.Run("and the snooze has not run out", func(t *testing.T) {
			layout := &city.Layout{Snoozed: map[string]time.Time{"a": now.Add(time.Minute)}}

			t.Run("it should set it aside", func(t *testing.T) {
				assert.True(t, layout.SetAside(session, now))
			})
		})

		t.Run("and the snooze has run out", func(t *testing.T) {
			layout := &city.Layout{Snoozed: map[string]time.Time{"a": now}}

			t.Run("it should not set it aside", func(t *testing.T) {
				assert.False(t, layout.SetAside(session, now))
			})
		})
	})
}
