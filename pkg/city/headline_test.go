package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 197: a city nobody is watching keeps its window title from the
// snapshot alone, and says what laying it out would say.
func TestHeadlineOf(t *testing.T) {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name  string
		shape func(*state.Snapshot, *city.Layout)
	}{
		{"when the city is the size of the real history", func(*state.Snapshot, *city.Layout) {}},
		{"when some sessions are packed up", func(snap *state.Snapshot, _ *city.Layout) {
			for i := 0; i < len(snap.Sessions); i += 3 {
				snap.Sessions[i].Packed = true
			}
		}},
		{"when a project is hidden", func(snap *state.Snapshot, l *city.Layout) {
			l.Hidden = map[string]bool{city.ProjectRoot(snap.Sessions[2].CWD): true}
		}},
		{"when a session needing you is snoozed", func(snap *state.Snapshot, l *city.Layout) {
			l.Snoozed = map[string]time.Time{snap.Sessions[2].ID: at.Add(time.Hour)}
		}},
		{"when a session has no folder", func(snap *state.Snapshot, _ *city.Layout) {
			snap.Sessions[2].CWD = ""
		}},
		{"when no session is on the map", func(snap *state.Snapshot, _ *city.Layout) {
			snap.Sessions = nil
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			layout := city.NewLayout()
			snap := realSized(at)
			c.shape(&snap, layout)
			s := city.NewScene(layout)
			s.SetClock(func() time.Time { return at }, time.UTC)
			s.Resize(1600, 900)
			read := s.HeadlineOf(snap)
			s.SetSnapshot(snap)

			t.Run("it should read the headline laying it out gives", func(t *testing.T) {
				assert.Equal(t, s.Summary().Headline(), read)
			})
		})
	}
}
