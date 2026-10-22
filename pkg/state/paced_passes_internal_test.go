package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// workingStats is an ingest that works 100 ms on the pace's clock
// between each of its yields.
type workingStats struct {
	clock  *fakeClock
	yields int
}

func (w *workingStats) Ingest(_ string, _ time.Time, yield func()) error {
	for range w.yields {
		w.clock.at = w.clock.at.Add(100 * time.Millisecond)
		yield()
	}
	return nil
}

func (w *workingStats) Hefts() (map[string]claude.Usage, error) {
	return map[string]claude.Usage{}, nil
}

func (w *workingStats) MovedHefts() (map[string]claude.Usage, []string, error) {
	return map[string]claude.Usage{}, nil, nil
}

// onOwnThread runs a pass on a goroutine of its own: a pass locks its
// thread and lowers its priority, and the test's goroutine may be on
// the main thread, which Go keeps when a locked goroutine exits.
func onOwnThread(pass func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		pass()
	}()
	<-done
}

// Item 159.
func TestPacedPasses(t *testing.T) {
	t.Run("when the stored-heft pass ingests and the ingest yields three times", func(t *testing.T) {
		c := &fakeClock{at: time.Unix(0, 0)}
		h := &storedHefts{src: &workingStats{clock: c, yields: 3}, yield: func() {}, pace: c.pace(0.5), running: true}
		dir := t.TempDir()
		onOwnThread(func() { h.pass(dir, time.Unix(0, 0)) })

		t.Run("it should pace at every yield", func(t *testing.T) {
			assert.Len(t, c.slept, 3)
		})
	})

	t.Run("when the whole read reads two stopped transcripts", func(t *testing.T) {
		c := &fakeClock{at: time.Unix(0, 0)}
		todo := []stoppedFile{{path: "a.jsonl"}, {path: "b.jsonl"}}
		w := &wholeRead{hefts: map[string]cachedHeft{}, pace: c.pace(0.5), running: true}
		w.readFile = func(string) (int64, bool) {
			c.at = c.at.Add(100 * time.Millisecond)
			return 0, true
		}
		onOwnThread(func() { w.pass(todo) })

		t.Run("it should pace after each one", func(t *testing.T) {
			assert.Len(t, c.slept, 2)
		})
	})
}
