package daemon_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/daemon"
)

// Item 205: a rescan's cost is its own thread's CPU time, so the
// machine's other work and the daemon's other goroutines do not count.
func TestThreadCPU(t *testing.T) {
	t.Run("when the thread spins for 100 ms", func(t *testing.T) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		before := daemon.ThreadCPU()
		spin := 0
		for start := time.Now(); time.Since(start) < 100*time.Millisecond; {
			spin++
		}
		used := daemon.ThreadCPU() - before

		t.Run("it should count most of it", func(t *testing.T) {
			assert.GreaterOrEqual(t, used, 25*time.Millisecond, "spun %d times", spin)
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
