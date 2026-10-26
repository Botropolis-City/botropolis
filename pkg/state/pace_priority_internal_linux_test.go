package state

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// classStats is an ingest that notes the I/O class it ran in.
type classStats struct {
	class int
}

func (c *classStats) Ingest(string, time.Time, func()) error {
	c.class = ioClassOfThisThread()
	return nil
}

func (c *classStats) Hefts() (map[string]claude.Usage, error) {
	return map[string]claude.Usage{}, nil
}

func (c *classStats) MovedHefts() (map[string]claude.Usage, []string, error) {
	return map[string]claude.Usage{}, nil, nil
}

// Item 254: a capture's read waited on the stored-heft pass, whose
// SQLite commits each fsync, and in the idle I/O class a disk whose
// scheduler honours it never served them while the other cities wrote
// their frames. A capture waits on the read and nothing else does
// (item 240), so only a pass paced for part of the clock is lowered.
func TestAPaceLowersOnlyAPacedPass(t *testing.T) {
	lowered := func(fraction float64) (before, after threadPriority) {
		p := &pace{fraction: fraction}
		onFreshThread(func() threadPriority {
			tid := syscall.Gettid()
			before = threadPriority{nice: niceOf(tid), ioClass: ioClassOfThisThread()}
			p.lower()
			after = threadPriority{nice: niceOf(tid), ioClass: ioClassOfThisThread()}
			return after
		})
		return before, after
	}

	t.Run("when a pace is for part of the clock, and its pass lowers its priority", func(t *testing.T) {
		_, after := lowered(backgroundShare)

		t.Run("it should be in the idle I/O class", func(t *testing.T) {
			assert.Equal(t, ioprioClassIdle, after.ioClass)
		})
	})

	t.Run("when a pace is for the whole clock, and its pass lowers its priority", func(t *testing.T) {
		before, after := lowered(1)

		t.Run("it should keep the thread's I/O class", func(t *testing.T) {
			assert.Equal(t, before.ioClass, after.ioClass)
		})
		t.Run("it should keep the thread's nice value", func(t *testing.T) {
			assert.Equal(t, before.nice, after.nice)
		})
	})
}

func TestAFullSpeedLoaderKeepsItsPriority(t *testing.T) {
	t.Run("when a loader reads at full speed, and the stored-heft pass ingests", func(t *testing.T) {
		src := &classStats{class: -1}
		l := NewLoader(t.TempDir(), Probes{}).WithStats(src, nil).WithFullSpeed()
		l.stored.running = true
		dir := t.TempDir()
		onOwnThread(func() { l.stored.pass(dir, time.Unix(0, 0), true, nil, false) })

		t.Run("it should ingest outside the idle I/O class", func(t *testing.T) {
			assert.NotEqual(t, ioprioClassIdle, src.class)
		})
	})

	t.Run("when a loader reads at full speed, statistics are off, and the whole read reads a stopped transcript", func(t *testing.T) {
		l := NewLoader(t.TempDir(), Probes{}).WithWholeRead(nil).WithFullSpeed()
		l.whole.running = true
		class := -1
		l.whole.readFile = func(string) (int64, bool) {
			class = ioClassOfThisThread()
			return 0, true
		}
		onOwnThread(func() { l.whole.pass([]stoppedFile{{path: "a.jsonl"}}) })

		t.Run("it should read outside the idle I/O class", func(t *testing.T) {
			assert.NotEqual(t, ioprioClassIdle, class)
		})
	})
}
