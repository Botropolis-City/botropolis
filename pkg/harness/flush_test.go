package harness_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/harness"
	"botropolis.city/botropolis/pkg/state"
)

type flushing struct {
	flushes int
	err     error
}

func (f *flushing) Name() string                               { return "flushing" }
func (f *flushing) Load(now time.Time) (state.Snapshot, error) { return state.Snapshot{At: now}, nil }
func (f *flushing) WatchDirs() []string                        { return nil }
func (f *flushing) Flush() error {
	f.flushes++
	return f.err
}

type plain struct{}

func (plain) Name() string                               { return "plain" }
func (plain) Load(now time.Time) (state.Snapshot, error) { return state.Snapshot{At: now}, nil }
func (plain) WatchDirs() []string                        { return nil }

// Item 205: the daemon flushes every harness that holds unsaved state
// when it stops.
func TestMultiFlush(t *testing.T) {
	t.Run("when harnesses that flush sit beside one that does not", func(t *testing.T) {
		first, second := &flushing{}, &flushing{}
		err := harness.NewMulti(first, plain{}, second).Flush()

		t.Run("it should flush the first", func(t *testing.T) {
			assert.Equal(t, 1, first.flushes)
		})

		t.Run("it should flush the second", func(t *testing.T) {
			assert.Equal(t, 1, second.flushes)
		})

		t.Run("it should report no error", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	t.Run("when one harness fails to flush", func(t *testing.T) {
		failing, after := &flushing{err: errors.New("disk full")}, &flushing{}
		err := harness.NewMulti(failing, after).Flush()

		t.Run("it should still flush the others", func(t *testing.T) {
			assert.Equal(t, 1, after.flushes)
		})

		t.Run("it should report the failure", func(t *testing.T) {
			assert.ErrorContains(t, err, "disk full")
		})
	})
}
