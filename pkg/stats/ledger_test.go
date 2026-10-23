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

	// Item 215: a re-read costs about 300 ms on the real history, so a
	// window is read again only when the store changed, at most every
	// LedgerCache, and every LedgerAging all the same, because messages
	// leave the window while nothing is written.
	t.Run("when a window was read", func(t *testing.T) {
		const aged = "2026-10-18T12:01:00.000Z"
		read := func(t *testing.T) (*fixture, *stats.Ledger) {
			f := newFixture(t)
			path := filepath.Join(t.TempDir(), "stats.db")
			f.store = open(t, path)
			f.seed(
				msg("msg_aged", aged, lib, usage{Output: 1}),
				msg("msg_1", "2026-10-19T11:00:00.000Z", lib, usage{Output: 2}),
			)
			l := newLedger(t, path)
			_, err := l.Window(t0, 24, nil)
			require.NoError(t, err)
			return f, l
		}
		output := func(t *testing.T, l *stats.Ledger, at time.Time) int64 {
			w, err := l.Window(at, 24, nil)
			require.NoError(t, err)
			return w.Counters.Output
		}

		t.Run("and the store then changed", func(t *testing.T) {
			changed := func(t *testing.T) *stats.Ledger {
				f, l := read(t)
				f.transcript(sessionB, reply{session: sessionB, id: "msg_2", at: "2026-10-19T11:30:00.000Z", cwd: lib, usage: usage{Output: 4}}.line())
				f.ingest()
				return l
			}

			t.Run("and it is asked again just inside LedgerCache, it should give the first answer", func(t *testing.T) {
				assert.Equal(t, int64(3), output(t, changed(t), t0.Add(stats.LedgerCache-time.Millisecond)))
			})

			t.Run("and it is asked again LedgerCache later, it should read the store again", func(t *testing.T) {
				assert.Equal(t, int64(7), output(t, changed(t), t0.Add(stats.LedgerCache)))
			})
		})

		t.Run("and the store did not change", func(t *testing.T) {
			t.Run("and it is asked again 2 minutes later, it should give the first answer, the aged message still counted", func(t *testing.T) {
				_, l := read(t)
				assert.Equal(t, int64(3), output(t, l, t0.Add(2*time.Minute)))
			})

			t.Run("and it is asked again LedgerAging later, it should read the store again, the aged message gone", func(t *testing.T) {
				_, l := read(t)
				assert.Equal(t, int64(2), output(t, l, t0.Add(stats.LedgerAging)))
			})
		})
	})

	t.Run("when LedgerCache and LedgerAging are set", func(t *testing.T) {
		t.Run("a window should be read at most every 30 s", func(t *testing.T) {
			assert.Equal(t, 30*time.Second, stats.LedgerCache)
		})

		t.Run("an unchanged window should be read again every 5 minutes", func(t *testing.T) {
			assert.Equal(t, 5*time.Minute, stats.LedgerAging)
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
