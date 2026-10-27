package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 280: the clinic stands for what needs you now, and what is in
// trouble, and clicking it lists them.
func TestClinic(t *testing.T) {
	needing := func(id string, since time.Duration) state.Session {
		s := session(id, cinders, state.NeedsYou)
		s.LastActivity = now.Add(-since)
		return s
	}
	erred := func(s state.Session, ago time.Duration) state.Session {
		s.APIErrors = 2
		s.LastErrorAt = now.Add(-ago)
		s.LastActivity = s.LastErrorAt
		return s
	}
	asking := needing("asking", 20*time.Minute)
	asking.Tool, asking.Subject = "AskUserQuestion", "which key?"
	recovered := erred(session("recovered", mullet, state.Working), 30*time.Minute)
	recovered.LastActivity = now.Add(-time.Minute)
	resting := erred(session("resting", mullet, state.Resting), 3*time.Minute)
	packed := erred(session("packed", mullet, state.Working), 3*time.Minute)
	packed.Packed = true
	both := erred(needing("both", 5*time.Minute), 5*time.Minute)

	scene := func(t *testing.T, layout *city.Layout, sessions ...state.Session) *city.Scene {
		t.Helper()
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(sessions...))
		return s
	}
	type row struct{ ID, Section, Reason string }
	rows := func(s *city.Scene) []row {
		var out []row
		for _, r := range s.Clinic() {
			out = append(out, row{r.Building.Session.ID, string(r.Section), r.Reason})
		}
		return out
	}

	t.Run("when some sessions need you and others' last record is an API error", func(t *testing.T) {
		s := scene(t, city.NewLayout(),
			needing("recent", 12*time.Minute), asking, needing("oldest", 3*time.Hour),
			erred(session("hours", mullet, state.Working), 2*time.Hour),
			erred(session("minutes", botropolis, state.Waiting), 4*time.Minute),
			session("fine", mullet, state.Working), recovered, resting, packed, both)
		got := rows(s)

		t.Run("it should list the sessions needing you first, longest waiting first, then the troubled ones, most recent first", func(t *testing.T) {
			var ids []string
			for _, r := range got {
				ids = append(ids, r.ID)
			}
			assert.Equal(t, []string{"oldest", "asking", "recent", "both", "minutes", "hours"}, ids)
		})

		t.Run("it should give each row its section and its reason", func(t *testing.T) {
			assert.Equal(t, []row{
				{"oldest", "needs you", "waiting 3h00m"},
				{"asking", "needs you", "AskUserQuestion which key?"},
				{"recent", "needs you", "waiting 12m"},
				{"both", "needs you", "waiting 5m"},
				{"minutes", "in trouble", "API error 4m ago"},
				{"hours", "in trouble", "API error 2h00m ago"},
			}, got)
		})

		t.Run("it should count exactly its rows", func(t *testing.T) {
			assert.Equal(t, len(got), s.ClinicCount())
		})

		t.Run("and the clinic is hovered", func(t *testing.T) {
			card, ok := s.CardFor(city.Hit{Landmark: city.LandmarkClinic})
			require.True(t, ok)

			t.Run("its card should say how many need you and how many are in trouble", func(t *testing.T) {
				assert.Equal(t, "4 need you · 2 in trouble", card.Lines[0])
			})
		})
	})

	t.Run("when a session needing you was set aside", func(t *testing.T) {
		layout := city.NewLayout()
		layout.Snoozed = map[string]time.Time{"recent": now.Add(time.Hour)}
		s := scene(t, layout, needing("recent", 12*time.Minute), needing("oldest", 3*time.Hour))

		t.Run("it should not list it", func(t *testing.T) {
			assert.Equal(t, []row{{"oldest", "needs you", "waiting 3h00m"}}, rows(s))
		})
	})

	t.Run("when nothing needs you and nothing is in trouble", func(t *testing.T) {
		s := scene(t, city.NewLayout(), session("fine", mullet, state.Working))

		t.Run("it should count none", func(t *testing.T) {
			assert.Zero(t, s.ClinicCount())
		})

		t.Run("its card should say nobody needs you", func(t *testing.T) {
			card, ok := s.CardFor(city.Hit{Landmark: city.LandmarkClinic})
			require.True(t, ok)
			assert.Equal(t, "nobody needs you and nothing is in trouble", card.Lines[0])
		})
	})

	t.Run("when the plaza is laid out", func(t *testing.T) {
		s := scene(t, city.NewLayout(), session("fine", mullet, state.Working))
		c := s.City()
		require.Positive(t, c.Clinic.Rect.Area(), "the clinic stands on the plaza")

		t.Run("it should answer a point on its plot as the clinic", func(t *testing.T) {
			assert.Equal(t, city.LandmarkClinic, c.At(c.Clinic.Rect.Center()).Landmark)
		})

		t.Run("its kind should be the clinic", func(t *testing.T) {
			assert.Equal(t, city.KindClinic, city.KindOf(city.Hit{Landmark: city.LandmarkClinic}))
		})
	})
}
