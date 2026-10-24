package stats_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

// Item 242: when the history's first read finishes, the city asks its
// ledger to read fresh, so it never shows a window cached partway
// through that read; the 30 s reuse holds for every ask after.
func TestALedgerReadsFreshWhenAsked(t *testing.T) {
	t0 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	read := func(t *testing.T) (*fixture, *stats.Ledger) {
		f := newFixture(t)
		path := filepath.Join(t.TempDir(), "stats.db")
		f.store = open(t, path)
		f.seed(msg("msg_1", "2026-10-19T11:00:00.000Z", lib, usage{Output: 2}))
		l := newLedger(t, path)
		_, err := l.Window(t0, 24, nil)
		require.NoError(t, err)
		_, err = l.SessionCosts(t0)
		require.NoError(t, err)
		return f, l
	}
	write := func(f *fixture, id, at string, output int64) {
		f.transcript(sessionB, reply{session: sessionB, id: id, at: at, cwd: lib, usage: usage{Output: output}}.line())
		f.ingest()
	}
	output := func(t *testing.T, l *stats.Ledger, at time.Time) int64 {
		w, err := l.Window(at, 24, nil)
		require.NoError(t, err)
		return w.Counters.Output
	}

	t.Run("when a window has been read, and messages are written within its 30 s", func(t *testing.T) {
		t.Run("and the ledger is asked to read fresh", func(t *testing.T) {
			f, l := read(t)
			write(f, "msg_2", "2026-10-19T11:30:00.000Z", 4)
			l.ReadFresh()

			t.Run("it should answer with the messages written", func(t *testing.T) {
				assert.Equal(t, int64(6), output(t, l, t0.Add(time.Second)))
			})
		})

		t.Run("and it is not asked", func(t *testing.T) {
			f, l := read(t)
			write(f, "msg_2", "2026-10-19T11:30:00.000Z", 4)

			t.Run("it should answer from its cache", func(t *testing.T) {
				assert.Equal(t, int64(2), output(t, l, t0.Add(time.Second)))
			})
		})

		t.Run("and it is asked to read fresh, answers, and more are written within 30 s", func(t *testing.T) {
			f, l := read(t)
			write(f, "msg_2", "2026-10-19T11:30:00.000Z", 4)
			l.ReadFresh()
			output(t, l, t0.Add(time.Second))
			write(f, "msg_3", "2026-10-19T11:40:00.000Z", 8)

			t.Run("it should answer from the fresh read's cache", func(t *testing.T) {
				assert.Equal(t, int64(6), output(t, l, t0.Add(2*time.Second)))
			})
		})
	})

	t.Run("when session costs have been read, a session's messages are written within the reuse, and the ledger is asked to read fresh", func(t *testing.T) {
		f, l := read(t)
		write(f, "msg_2", "2026-10-19T11:30:00.000Z", 4)
		l.ReadFresh()
		costs, err := l.SessionCosts(t0.Add(time.Second))
		require.NoError(t, err)

		t.Run("it should price them again", func(t *testing.T) {
			assert.Contains(t, costs, sessionB)
		})
	})
}
