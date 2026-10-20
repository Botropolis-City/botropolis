package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestSceneSummary(t *testing.T) {
	waiting := session("b", cinders, state.NeedsYou)
	waiting.Title = "Waiting"
	snoozed := session("a", cinders, state.NeedsYou)
	snoozed.Title = "Snoozed"
	scene := func(layout *city.Layout, at time.Time, sessions ...state.Session) *city.Scene {
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(sessions...))
		return s
	}
	snoozedLayout := func() *city.Layout {
		layout := city.NewLayout()
		layout.Snoozed = map[string]time.Time{"a": now.Add(time.Hour)}
		return layout
	}

	t.Run("when one session needs you and another needing you is snoozed", func(t *testing.T) {
		t.Run("and the snooze has not run out", func(t *testing.T) {
			s := scene(snoozedLayout(), now, snoozed, waiting)
			summary := s.Summary()

			t.Run("it should count one needing you", func(t *testing.T) {
				assert.Equal(t, 1, summary.NeedsYou)
			})

			t.Run("it should count one set aside", func(t *testing.T) {
				assert.Equal(t, 1, summary.SetAside)
			})

			t.Run("it should headline one needing you and one set aside", func(t *testing.T) {
				assert.Equal(t, "1 needs you · 1 set aside", summary.Headline())
			})

			t.Run("it should list only the unsnoozed session on the needs-you card", func(t *testing.T) {
				assert.Equal(t, []string{"Waiting"}, s.StateCard(state.NeedsYou).Lines)
			})

			t.Run("it should jump to the unsnoozed session from the needs-you chip", func(t *testing.T) {
				b := s.JumpTo(state.NeedsYou)
				require.NotNil(t, b)
				assert.Equal(t, "Waiting", b.Session.Title)
			})

			t.Run("and the Attention lens is up", func(t *testing.T) {
				s.SelectLens(city.LensAttention)

				t.Run("it should count only the sessions not set aside", func(t *testing.T) {
					assert.Equal(t, "1 session needs you", s.LensPanel().Aggregate)
				})
			})
		})

		t.Run("and the snooze has run out", func(t *testing.T) {
			s := scene(snoozedLayout(), now.Add(time.Hour), waiting, snoozed)

			t.Run("it should headline two needing you", func(t *testing.T) {
				assert.Equal(t, "2 need you", s.Summary().Headline())
			})

			t.Run("it should list both sessions on the needs-you card", func(t *testing.T) {
				assert.ElementsMatch(t, []string{"Waiting", "Snoozed"}, s.StateCard(state.NeedsYou).Lines)
			})
		})
	})

	t.Run("when the only session is set aside", func(t *testing.T) {
		s := scene(snoozedLayout(), now, snoozed)
		s.SelectLens(city.LensAttention)

		t.Run("it should tell the Attention lens nobody is waiting", func(t *testing.T) {
			assert.Equal(t, "Nobody is waiting on you; tab finds the next one when someone is.", s.LensPanel().Do)
		})
	})

	t.Run("when a session marked done has not moved since, and is not packed", func(t *testing.T) {
		layout := city.NewLayout()
		layout.Done = map[string]time.Time{"a": snoozed.LastActivity}
		s := scene(layout, now, snoozed, waiting)

		t.Run("it should count it set aside", func(t *testing.T) {
			assert.Equal(t, 1, s.Summary().SetAside)
		})
	})

	t.Run("when triage has set nothing aside", func(t *testing.T) {
		s := scene(city.NewLayout(), now, snoozed, waiting)

		t.Run("it should count every needs-you session as needing you", func(t *testing.T) {
			assert.Equal(t, 2, s.Summary().NeedsYou)
		})
	})
}
