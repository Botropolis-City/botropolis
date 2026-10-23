package measure

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func processCPU(t *testing.T) time.Duration {
	t.Helper()
	var u syscall.Rusage
	require.NoError(t, syscall.Getrusage(syscall.RUSAGE_SELF, &u))
	return time.Duration(u.Utime.Nano() + u.Stime.Nano())
}

// Item 209: a gate that means CPU counts the thread's CPU, so a busy
// machine that keeps the thread waiting for a core does not move it.
func TestThread(t *testing.T) {
	t.Run("when the measured work sleeps for 50 ms", func(t *testing.T) {
		spent := Thread(func() { time.Sleep(50 * time.Millisecond) }).CPU

		t.Run("it should count under 5 ms of CPU", func(t *testing.T) {
			assert.Less(t, spent, 5*time.Millisecond)
		})
	})

	t.Run("when the measured work spins for 50 ms", func(t *testing.T) {
		before := processCPU(t)
		spent := Thread(func() {
			for at := time.Now(); time.Since(at) < 50*time.Millisecond; {
			}
		}).CPU
		process := processCPU(t) - before

		t.Run("it should count at least 80% of the CPU the process spent meanwhile", func(t *testing.T) {
			assert.GreaterOrEqual(t, float64(spent), 0.8*float64(process), "thread %v, process %v", spent, process)
		})
	})
}
