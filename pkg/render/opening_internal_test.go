package render

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 159: every wait before the city is drawn shows real progress.
func TestOpeningFor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		wait   openingWait
		line   string
		fill   float64
		showed bool
	}{
		{"when the city is ready within 300 ms", openingWait{pagesUp: 1, pages: 2, waited: 200 * time.Millisecond}, "", 0, false},
		{"when one of 2 pages is up after 400 ms", openingWait{pagesUp: 1, pages: 2, waited: 400 * time.Millisecond}, "Opening the city", 0.5, true},
		{"when three of 6 pages are up after 400 ms", openingWait{pagesUp: 3, pages: 6, waited: 400 * time.Millisecond}, "Opening the city", 0.5, true},
		{"when the pages are up and the daemon has read 40 of 120 sessions", openingWait{pagesUp: 2, pages: 2, waited: time.Second, load: state.LoadProgress{Done: 40, Total: 120}}, "Reading your sessions · 40 of 120", 1.0 / 3, true},
		{"when the pages are up and nothing says how far the load has got", openingWait{pagesUp: 2, pages: 2, waited: time.Second}, "Reading your sessions", ui.NoFill, true},
		{"when the city has opened", openingWait{pagesUp: 2, pages: 2, waited: time.Second, opened: true}, "", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line, fill, showed := openingFor(tc.wait)

			t.Run("it should show the panel only when there is a wait to show", func(t *testing.T) {
				assert.Equal(t, tc.showed, showed)
			})

			t.Run("it should say what the wait is", func(t *testing.T) {
				assert.Equal(t, tc.line, line)
			})

			t.Run("it should fill as far as it has got", func(t *testing.T) {
				assert.InDelta(t, tc.fill, fill, 1e-9)
			})
		})
	}
}

func TestWatchLoading(t *testing.T) {
	t.Run("when the city waits on a daemon that has read 40 of 120 sessions", func(t *testing.T) {
		var opened atomic.Bool
		var asks atomic.Int64
		var got atomic.Pointer[state.LoadProgress]
		done := make(chan struct{})
		go func() {
			defer close(done)
			watchLoading(time.Millisecond, opened.Load, func() (state.LoadProgress, error) {
				asks.Add(1)
				return state.LoadProgress{Done: 40, Total: 120}, nil
			}, got.Store)
		}()
		require.Eventually(t, func() bool { return got.Load() != nil }, time.Second, time.Millisecond)

		t.Run("it should keep what the daemon said", func(t *testing.T) {
			assert.Equal(t, state.LoadProgress{Done: 40, Total: 120}, *got.Load())
		})

		t.Run("and the city opens", func(t *testing.T) {
			opened.Store(true)
			select {
			case <-done:
			case <-time.After(time.Second):
				require.FailNow(t, "it kept asking after the city opened")
			}
			before := asks.Load()
			time.Sleep(10 * time.Millisecond)

			t.Run("it should stop asking", func(t *testing.T) {
				assert.Equal(t, before, asks.Load())
			})
		})
	})
}
