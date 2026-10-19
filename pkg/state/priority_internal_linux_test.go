package state

import (
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

type threadPriority struct {
	before, after int
	ioClass       int
}

// onFreshThread runs f on a thread of its own, which is thrown away
// afterwards so no other test inherits what f did to it.
func onFreshThread(f func() threadPriority) threadPriority {
	got := make(chan threadPriority)
	go func() {
		runtime.LockOSThread()
		got <- f()
	}()
	return <-got
}

func ioClassOfThisThread() int {
	prio, _, _ := syscall.Syscall(unix.SYS_IOPRIO_GET, ioprioWhoProcess, uintptr(syscall.Gettid()), 0)
	return int(prio) >> ioprioClassShift
}

// Item 138: a thread at nice 19 made the whole daemon wait for it at
// every garbage collection on a busy machine.
func TestBackgroundPriority(t *testing.T) {
	t.Run("when a thread is given background priority", func(t *testing.T) {
		p := onFreshThread(func() threadPriority {
			var p threadPriority
			p.before, _ = syscall.Getpriority(syscall.PRIO_PROCESS, syscall.Gettid())
			backgroundPriority()
			p.after, _ = syscall.Getpriority(syscall.PRIO_PROCESS, syscall.Gettid())
			p.ioClass = ioClassOfThisThread()
			return p
		})

		t.Run("it should keep the process's CPU priority", func(t *testing.T) {
			assert.Equal(t, p.before, p.after)
		})

		t.Run("it should be in the idle I/O class", func(t *testing.T) {
			assert.Equal(t, ioprioClassIdle, p.ioClass)
		})
	})
}
