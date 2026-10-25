package daemon_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"botropolis.city/botropolis/pkg/daemon"
)

const (
	threadSpin      = 50 * time.Millisecond
	threadTimeout   = 30 * time.Second
	threadTolerance = 5 * time.Millisecond
)

func threadClock(t *testing.T) time.Duration {
	var ts unix.Timespec
	require.NoError(t, unix.ClockGettime(unix.CLOCK_THREAD_CPUTIME_ID, &ts))
	return time.Duration(ts.Nano())
}

func burn(start <-chan struct{}, done chan<- struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)
	<-start
	var ts unix.Timespec
	clock := func() time.Duration {
		_ = unix.ClockGettime(unix.CLOCK_THREAD_CPUTIME_ID, &ts)
		return time.Duration(ts.Nano())
	}
	deadline := time.Now().Add(threadTimeout)
	for from := clock(); clock()-from < threadSpin && time.Now().Before(deadline); {
	}
}

// Item 205: a rescan's cost is its own thread's CPU time, so the
// machine's other work and the daemon's other goroutines do not count.
// Item 225: the spin is counted on the thread's own clock rather than the
// wall's, so a loaded machine lengthens the test instead of failing it.
func TestThreadCPU(t *testing.T) {
	t.Run("when the thread spins until its own clock has counted 50 ms", func(t *testing.T) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		start, done := make(chan struct{}), make(chan struct{})
		go burn(start, done)

		before, from := daemon.ThreadCPU(), threadClock(t)
		close(start)
		began, spin, burnt := time.Now(), 0, false
		for !burnt || threadClock(t)-from < threadSpin {
			if time.Since(began) > threadTimeout {
				break
			}
			select {
			case <-done:
				burnt = true
			default:
			}
			spin++
		}
		to, after := threadClock(t), daemon.ThreadCPU()
		wall := time.Since(began)

		t.Run("it should not time out", func(t *testing.T) {
			assert.Less(t, wall, threadTimeout, "spun %d times", spin)
		})

		t.Run("it should report the thread's own clock", func(t *testing.T) {
			assert.InDelta(t, to-from, after-before, float64(threadTolerance), "reference %v, reported %v", to-from, after-before)
		})
	})

	t.Run("when the thread sleeps for 100 ms", func(t *testing.T) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		before := daemon.ThreadCPU()
		time.Sleep(100 * time.Millisecond)
		used := daemon.ThreadCPU() - before

		t.Run("it should count almost none of it", func(t *testing.T) {
			assert.Less(t, used, 20*time.Millisecond)
		})
	})
}
