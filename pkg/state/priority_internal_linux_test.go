package state

import (
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

type threadPriority struct {
	nice, otherNice int
	ioClass         int
}

// niceOf is a thread's nice value: the raw system call answers 20 less
// it, so that it is never negative.
func niceOf(tid int) int {
	raw, _ := syscall.Getpriority(syscall.PRIO_PROCESS, tid)
	return 20 - raw
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
// every garbage collection on a busy machine. Item 159: once item 157
// cut the collections, nice 10 measured the same as none beside four
// busy loops on one core (101-135 ms, against 101-126 ms), and nice 19
// still drifted (105-271 ms), so the pass goes to 10.
func TestBackgroundPriority(t *testing.T) {
	t.Run("when a thread is given background priority", func(t *testing.T) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		ownThread := syscall.Gettid()
		own := niceOf(ownThread)
		p := onFreshThread(func() threadPriority {
			var p threadPriority
			backgroundPriority()
			p.nice = niceOf(syscall.Gettid())
			p.otherNice = niceOf(ownThread)
			p.ioClass = ioClassOfThisThread()
			return p
		})

		t.Run("it should lower the thread's CPU priority to nice 10, unless the process is lower already", func(t *testing.T) {
			assert.Equal(t, max(own, 10), p.nice)
		})

		t.Run("it should leave the process's other threads where they were", func(t *testing.T) {
			assert.Equal(t, own, p.otherNice)
		})

		t.Run("it should be in the idle I/O class", func(t *testing.T) {
			assert.Equal(t, ioprioClassIdle, p.ioClass)
		})
	})
}
