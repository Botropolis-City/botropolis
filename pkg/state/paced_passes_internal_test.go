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
		onOwnThread(func() { h.pass(dir, time.Unix(0, 0), true, nil, false) })

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

	// Item 240: a capture waits on the read and nothing else does, so it
	// reads at full speed.
	t.Run("when a loader reads at full speed", func(t *testing.T) {
		onClock := func(p *pace, c *fakeClock) {
			p.now, p.sleep = c.now, c.sleep
			p.start()
		}

		t.Run("and the stored-heft pass ingests, yielding three times", func(t *testing.T) {
			c := &fakeClock{at: time.Unix(0, 0)}
			l := NewLoader(t.TempDir(), Probes{}).WithStats(&workingStats{clock: c, yields: 3}, nil).WithFullSpeed()
			onClock(l.stored.pace, c)
			l.stored.running = true
			dir := t.TempDir()
			onOwnThread(func() { l.stored.pass(dir, time.Unix(0, 0), true, nil, false) })

			t.Run("it should not sleep", func(t *testing.T) {
				assert.Empty(t, c.slept)
			})
		})

		t.Run("and statistics are off, and the whole read reads two stopped transcripts", func(t *testing.T) {
			c := &fakeClock{at: time.Unix(0, 0)}
			l := NewLoader(t.TempDir(), Probes{}).WithWholeRead(nil).WithFullSpeed()
			onClock(l.whole.pace, c)
			l.whole.running = true
			l.whole.readFile = func(string) (int64, bool) {
				c.at = c.at.Add(100 * time.Millisecond)
				return 0, true
			}
			onOwnThread(func() { l.whole.pass([]stoppedFile{{path: "a.jsonl"}, {path: "b.jsonl"}}) })

			t.Run("it should not sleep", func(t *testing.T) {
				assert.Empty(t, c.slept)
			})
		})

		// The later read's pass times reads of real files, so its pace is
		// checked as given; TestPace shows a whole clock never sleeps.
		t.Run("and the later read is made", func(t *testing.T) {
			l := NewLoader(t.TempDir(), Probes{}).WithStats(&workingStats{clock: &fakeClock{}}, nil).WithFullSpeed()

			t.Run("it should pace it for the whole clock", func(t *testing.T) {
				assert.Equal(t, 1.0, l.later.pace.fraction)
			})
		})
	})
}
