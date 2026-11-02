package measure

import (
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// Ebitengine's main thread is locked to its goroutine, and the layout
// timer runs on it, so Thread must leave a lock it found in place.
func TestThreadLeavesALock(t *testing.T) {
	t.Run("when it is called on a goroutine already locked to its thread, and that goroutine then exits", func(t *testing.T) {
		// The process's main thread is parked rather than ended when a
		// locked goroutine exits on it, so a run that lands there tries
		// again, on a thread that can end.
		ran := func() int {
			tid := make(chan int)
			go func() {
				runtime.LockOSThread()
				Thread(func() {})
				tid <- unix.Gettid()
			}()
			return <-tid
		}
		on := ran()
		for tries := 0; on == os.Getpid() && tries < 5; tries++ {
			on = ran()
		}
		require.NotEqual(t, os.Getpid(), on, "every try ran on the main thread, which a locked exit would have parked: the lock was dropped")
		task := "/proc/self/task/" + strconv.Itoa(on)

		t.Run("it should leave the goroutine locked, so its thread ends with it", func(t *testing.T) {
			assert.Eventually(t, func() bool { _, err := os.Stat(task); return os.IsNotExist(err) }, 5*time.Second, 10*time.Millisecond)
		})
	})
}

// Item 330: a reading's thread CPU depends on the kind of core it ran on,
// so a measure says which core it ended on.
func TestThreadNamesItsCore(t *testing.T) {
	for _, core := range []int{0, runtime.NumCPU() - 1} {
		t.Run("when the thread is pinned to core "+strconv.Itoa(core), func(t *testing.T) {
			var ended int
			done := make(chan struct{})
			go func() {
				defer close(done)
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				var only unix.CPUSet
				only.Set(core)
				require.NoError(t, unix.SchedSetaffinity(0, &only))
				ended = Thread(func() {}).EndedOn
			}()
			<-done

			t.Run("it should name that core", func(t *testing.T) {
				assert.Equal(t, core, ended)
			})
		})
	}
}
