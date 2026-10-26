package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

// readingLedger is a ledger whose store is partway through the history's
// first read until it is asked to read fresh: it answers from its cache
// until then, as a stats.Ledger does within its 30 s.
type readingLedger struct {
	fresh int
}

func (l *readingLedger) Window(time.Time, int, func(string) string, map[string]string) (stats.Window, error) {
	if l.fresh > 0 {
		return stats.Window{Cost: cost(150)}, nil
	}
	return stats.Window{Cost: cost(10)}, nil
}

func (l *readingLedger) SessionCosts(time.Time) (map[string]stats.Cost, error) {
	return map[string]stats.Cost{}, nil
}

func (l *readingLedger) Reason() string { return "" }

func (l *readingLedger) ReadFresh() { l.fresh++ }

// Item 242: when the history's first read finishes, the scene has its
// ledger read fresh, so neither a capture opening on that read nor a
// live city shows a window cached partway through it.
func TestTheSceneReadsTheLedgerFreshWhenItsHistoryIsRead(t *testing.T) {
	scene := func(openOnRead bool, l *readingLedger) *city.Scene {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetOpenOnRead(openOnRead)
		s.SetLedger(l)
		return s
	}
	read := func(read bool) state.Snapshot {
		snap := snapshot(session("a", cinders, state.Working))
		snap.HeftsRead = read
		return snap
	}

	for _, tc := range []struct {
		name       string
		openOnRead bool
	}{
		{"a scene opening on a read holds an unread snapshot", true},
		{"a live scene showed an unread snapshot", false},
	} {
		t.Run("when "+tc.name+", and a read one arrives", func(t *testing.T) {
			s := scene(tc.openOnRead, &readingLedger{})
			s.SetSnapshot(read(false))
			s.SetSnapshot(read(true))

			t.Run("it should show the day's cost the store holds after the read", func(t *testing.T) {
				assert.Equal(t, cost(150), s.City().Plant.Day.Cost)
			})
		})
	}

	t.Run("when a read snapshot follows a read one", func(t *testing.T) {
		l := &readingLedger{}
		s := scene(false, l)
		s.SetSnapshot(read(false))
		s.SetSnapshot(read(true))
		s.SetSnapshot(read(true))

		t.Run("it should not have the ledger read fresh again", func(t *testing.T) {
			assert.Equal(t, 1, l.fresh)
		})
	})
}

var _ city.Ledger = (*readingLedger)(nil)
