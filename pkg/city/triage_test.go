package city_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestTriage(t *testing.T) {
	waiting := func(id, cwd string, since time.Duration) state.Session {
		s := session(id, cwd, state.NeedsYou)
		s.LastActivity = now.Add(-since)
		return s
	}
	walking := func(t *testing.T, layout *city.Layout) (*city.Scene, *time.Time) {
		t.Helper()
		clock := now
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(waiting("a", cinders, 30*time.Minute), waiting("b", botropolis, 2*time.Hour),
			session("c", mullet, state.Working), session("d", cinders, state.Waiting)))
		return s, &clock
	}
	ids := func(s *city.Scene) []string {
		var out []string
		for _, b := range s.TriageWalk() {
			out = append(out, b.Session.ID)
		}
		return out
	}
	current := func(s *city.Scene) string {
		step, ok := s.TriageStep()
		if !ok {
			return ""
		}
		return step.Building.Session.ID
	}

	t.Run("when sessions wait on you", func(t *testing.T) {
		s, _ := walking(t, city.NewLayout())

		t.Run("it should walk the sessions that need you", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"a", "b"}, ids(s))
		})

		t.Run("it should leave out the ones that are working", func(t *testing.T) {
			assert.NotContains(t, ids(s), "c")
		})

		t.Run("it should walk the longest-waiting first", func(t *testing.T) {
			assert.Equal(t, []string{"b", "a"}, ids(s))
		})
	})

	t.Run("when a step is taken", func(t *testing.T) {
		s, _ := walking(t, city.NewLayout())
		require.True(t, s.StartTriage())

		t.Run("it should select that session", func(t *testing.T) {
			assert.Equal(t, "b", s.Selected().Session.ID)
		})

		t.Run("it should centre the camera on it", func(t *testing.T) {
			in, size := s.Insets(), s.Size()
			want := city.Point{X: in.Left + (size.X-in.Left-in.Right)/2, Y: in.Top + (size.Y-in.Top-in.Bottom)/2}
			got := s.Camera().WorldToScreen(s.Selected().Rect.Center())
			assert.True(t, math.Abs(got.X-want.X) < 1 && math.Abs(got.Y-want.Y) < 1, "%v, want %v", got, want)
		})
	})

	t.Run("when n is pressed", func(t *testing.T) {
		s, _ := walking(t, city.NewLayout())
		s.StartTriage()
		s.TriageNext()
		second := current(s)
		s.TriageNext()

		t.Run("it should move to the next", func(t *testing.T) {
			assert.Equal(t, "a", second)
		})

		t.Run("and it was the last, it should come back to the first", func(t *testing.T) {
			assert.Equal(t, "b", current(s))
		})
	})

	t.Run("when d is pressed", func(t *testing.T) {
		layout := city.NewLayout()
		s, _ := walking(t, layout)
		s.StartTriage()
		s.TriageDone()

		t.Run("it should leave the walk", func(t *testing.T) {
			assert.Equal(t, []string{"a"}, ids(s))
		})

		t.Run("it should move on to the next", func(t *testing.T) {
			assert.Equal(t, "a", current(s))
		})

		t.Run("and the session does something new", func(t *testing.T) {
			again := waiting("b", botropolis, time.Minute)
			s.SetSnapshot(snapshot(waiting("a", cinders, 30*time.Minute), again))

			t.Run("it should come back", func(t *testing.T) {
				assert.Contains(t, ids(s), "b")
			})
		})
	})

	t.Run("when s is pressed and a time is chosen", func(t *testing.T) {
		s, clock := walking(t, city.NewLayout())
		s.StartTriage()
		require.NoError(t, s.TriageSnooze(now.Add(time.Hour)))

		t.Run("it should leave the walk", func(t *testing.T) {
			assert.Equal(t, []string{"a"}, ids(s))
		})

		t.Run("and the time passes", func(t *testing.T) {
			*clock = now.Add(time.Hour)

			t.Run("it should come back", func(t *testing.T) {
				assert.Contains(t, ids(s), "b")
			})
		})
	})

	t.Run("when s is given a time that has passed", func(t *testing.T) {
		s, _ := walking(t, city.NewLayout())
		s.StartTriage()

		t.Run("it should refuse it", func(t *testing.T) {
			assert.Error(t, s.TriageSnooze(now.Add(-time.Minute)))
		})
	})

	t.Run("when p is pressed", func(t *testing.T) {
		s, _ := walking(t, city.NewLayout())
		s.StartTriage()
		action, _ := s.TriageRest()

		t.Run("it should ask to stop the session", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionStop, SessionID: "b"}, action)
		})
	})

	t.Run("when enter is pressed", func(t *testing.T) {
		s, _ := walking(t, city.NewLayout())
		s.StartTriage()
		action, _ := s.TriageAct()

		t.Run("it should do the card's first action", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionAttach, SessionID: "b", Dir: botropolis}, action)
		})
	})

	t.Run("when nothing waits on you", func(t *testing.T) {
		s := scene(t, session("c", mullet, state.Working))

		t.Run("it should say so", func(t *testing.T) {
			assert.False(t, s.StartTriage())
		})
	})

	t.Run("when Botropolis restarts", func(t *testing.T) {
		path := t.TempDir() + "/layout.json"
		layout := city.NewLayout()
		s, _ := walking(t, layout)
		s.StartTriage()
		s.TriageDone()
		require.NoError(t, s.TriageSnooze(now.Add(time.Hour)))
		require.NoError(t, layout.Save(path))
		loaded, err := city.LoadLayout(path)
		require.NoError(t, err)
		again, _ := walking(t, loaded)

		t.Run("it should remember what was done and what is snoozed", func(t *testing.T) {
			assert.Empty(t, ids(again))
		})
	})
}

func TestParseSnooze(t *testing.T) {
	cases := []struct {
		text string
		want time.Time
	}{
		{"18:00", time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)},
		{"9:30", time.Date(2026, 9, 17, 9, 30, 0, 0, time.UTC)},
		{"tomorrow 9:00", time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)},
		{"Tomorrow 09:00", time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)},
		{"in 2h", now.Add(2 * time.Hour)},
		{"in 45m", now.Add(45 * time.Minute)},
		{"in 1h30m", now.Add(90 * time.Minute)},
		{"2026-09-20 14:00", time.Date(2026, 9, 20, 14, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run("when it is "+tc.text+", it should read it", func(t *testing.T) {
			got, err := city.ParseSnooze(tc.text, now)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	refused := []struct{ text, why string }{
		{"soon", "write a time like"},
		{"25:00", "write a time like"},
		{"", "write a time like"},
		{"07:00", "has passed"},
		{"2026-09-01 10:00", "has passed"},
		{"in 0m", "has passed"},
	}
	for _, tc := range refused {
		t.Run("when it is "+tc.text+", it should refuse it and say why", func(t *testing.T) {
			_, err := city.ParseSnooze(tc.text, now)
			assert.ErrorContains(t, err, tc.why)
		})
	}
}

func TestSnoozeChoices(t *testing.T) {
	labels := func(at time.Time) []string {
		var out []string
		for _, c := range city.SnoozeChoices(at) {
			out = append(out, c.Label)
		}
		return out
	}

	t.Run("when it is morning, it should offer an hour, this evening and tomorrow", func(t *testing.T) {
		assert.Equal(t, []string{"in an hour", "this evening at 18:00", "tomorrow at 09:00"}, labels(now))
	})

	t.Run("when the evening has begun, it should not offer this evening", func(t *testing.T) {
		assert.Equal(t, []string{"in an hour", "tomorrow at 09:00"}, labels(now.Add(11*time.Hour)))
	})
}
