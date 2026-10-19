package state

import (
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	ioprioWhoProcess = 1
	ioprioClassShift = 13
	ioprioClassIdle  = 3
)

// backgroundPriority puts the calling thread in the idle I/O class, so
// its reads wait behind everyone else's. Its CPU priority stays the
// process's: Go stops every thread for each garbage collection, and on
// a busy machine a thread at nice 19 held the whole daemon waiting for
// it, seconds at a time (item 138). The passes give way on the CPU by
// yielding to loads instead.
func backgroundPriority() {
	_, _, _ = syscall.Syscall(unix.SYS_IOPRIO_SET, ioprioWhoProcess, uintptr(syscall.Gettid()), ioprioClassIdle<<ioprioClassShift)
}
