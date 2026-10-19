package stats_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

func newLedger(t *testing.T, path string) *stats.Ledger {
	t.Helper()
	l := stats.NewLedger(path)
	t.Cleanup(l.Close)
	return l
}

func TestLedger(t *testing.T) {
	t0 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	t.Run("when there is no store yet", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stats.db")
		l := newLedger(t, path)
		_, ok := l.Store(t0)

		t.Run("it should give no store", func(t *testing.T) {
			assert.False(t, ok)
		})

		t.Run("it should say why", func(t *testing.T) {
			assert.Equal(t, "there are no statistics yet", l.Reason())
		})

		t.Run("and the store appears", func(t *testing.T) {
			w, err := stats.Open(path)
			require.NoError(t, err)
			require.NoError(t, w.Close())

			t.Run("and it is asked again before the retry", func(t *testing.T) {
				_, ok := l.Store(t0.Add(stats.LedgerRetry - time.Second))

				t.Run("it should not try again yet", func(t *testing.T) {
					assert.False(t, ok)
				})
			})

			t.Run("and it is asked again after the retry", func(t *testing.T) {
				_, ok := l.Store(t0.Add(stats.LedgerRetry))

				t.Run("it should open it", func(t *testing.T) {
					assert.True(t, ok)
				})

				t.Run("it should give no reason", func(t *testing.T) {
					assert.Equal(t, "", l.Reason())
				})
			})
		})
	})

	t.Run("when a window is asked for twice", func(t *testing.T) {
		f := newFixture(t)
		path := filepath.Join(t.TempDir(), "stats.db")
		f.store = open(t, path)
		f.seed(msg("msg_1", "2026-10-19T11:00:00.000Z", lib, usage{Output: 1}))
		l := newLedger(t, path)
		first, err := l.Window(t0, 24, nil)
		require.NoError(t, err)
		f.transcript(sessionB, reply{session: sessionB, id: "msg_2", at: "2026-10-19T11:30:00.000Z", cwd: lib, usage: usage{Output: 2}}.line())
		f.ingest()

		t.Run("and the second is within the cache's life", func(t *testing.T) {
			again, err := l.Window(t0.Add(stats.LedgerCache-time.Millisecond), 24, nil)
			require.NoError(t, err)

			t.Run("it should give the first answer", func(t *testing.T) {
				assert.Equal(t, first.Counters.Output, again.Counters.Output)
			})
		})

		t.Run("and the second is after it", func(t *testing.T) {
			again, err := l.Window(t0.Add(stats.LedgerCache), 24, nil)
			require.NoError(t, err)

			t.Run("it should read the store again", func(t *testing.T) {
				assert.Equal(t, int64(3), again.Counters.Output)
			})
		})
	})

	t.Run("when the store is paused, a window should be refused with the reason", func(t *testing.T) {
		l := newLedger(t, filepath.Join(t.TempDir(), "stats.db"))
		_, err := l.Window(t0, 24, nil)
		assert.ErrorIs(t, err, stats.ErrNoStore)
	})

	t.Run("when there is no ledger, it should say statistics are off", func(t *testing.T) {
		var l *stats.Ledger
		assert.Equal(t, "statistics are off", l.Reason())
	})
}
