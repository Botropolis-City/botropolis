package state_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// reportingStats is an ingest that reports done of known bytes and
// holds there until it is let go.
type reportingStats struct {
	done, known int64
	ingesting   chan struct{}
	release     chan struct{}
	finished    chan struct{}
}

func newReportingStats(done, known int64) *reportingStats {
	return &reportingStats{done: done, known: known, ingesting: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
}

func (r *reportingStats) Ingest(string, time.Time, func()) error {
	close(r.ingesting)
	<-r.release
	return nil
}

func (r *reportingStats) Hefts() (map[string]claude.Usage, error) {
	return map[string]claude.Usage{}, nil
}

func (r *reportingStats) MovedHefts() (map[string]claude.Usage, []string, error) {
	return map[string]claude.Usage{}, nil, nil
}

func (r *reportingStats) Progress() (int64, int64) { return r.done, r.known }

// Item 159: the city shows how far a first launch has got.
func TestLoaderProgress(t *testing.T) {
	statsLoader := func(t *testing.T, src *reportingStats) *state.Loader {
		t.Helper()
		loader := state.NewLoader(helpers.NewHome(t).Path, alive).WithStats(src, func() { close(src.finished) })
		_, err := loader.Load(now)
		require.NoError(t, err)
		<-src.ingesting
		return loader
	}

	t.Run("when the stored-heft pass is running and has 64 MiB to read", func(t *testing.T) {
		src := newReportingStats(16<<20, 64<<20)
		loader := statsLoader(t, src)
		t.Cleanup(func() { close(src.release) })

		t.Run("it should report how far it has got", func(t *testing.T) {
			assert.Equal(t, &state.ScanProgress{Done: 16 << 20, Known: 64 << 20}, loader.Progress())
		})

		t.Run("and the loader loads", func(t *testing.T) {
			snapshot, err := loader.Load(now)
			require.NoError(t, err)

			t.Run("it should put that progress on the snapshot", func(t *testing.T) {
				assert.Equal(t, &state.ScanProgress{Done: 16 << 20, Known: 64 << 20}, snapshot.Scan)
			})
		})
	})

	t.Run("when the stored-heft pass is running but has less than 32 MiB to read", func(t *testing.T) {
		src := newReportingStats(0, 1<<20)
		loader := statsLoader(t, src)
		t.Cleanup(func() { close(src.release) })

		t.Run("it should report nothing, since there is no wait to show", func(t *testing.T) {
			assert.Nil(t, loader.Progress())
		})
	})

	t.Run("when the pass has finished", func(t *testing.T) {
		src := newReportingStats(64<<20, 64<<20)
		loader := statsLoader(t, src)
		close(src.release)
		<-src.finished

		t.Run("it should report nothing", func(t *testing.T) {
			assert.Nil(t, loader.Progress())
		})
	})
}
