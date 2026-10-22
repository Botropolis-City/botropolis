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

// backgroundNice is the calling thread's CPU priority in a background
// pass. Go stops every thread for each garbage collection, and on a
// busy machine a thread at nice 19 held the whole daemon waiting for it,
// seconds at a time (item 138). With item 157's few collections, nice
// 10 measured the same as none beside four busy loops on one core, and
// nice 19 still drifted (item 159).
const backgroundNice = 10

// backgroundPriority puts the calling thread at backgroundNice, unless
// the process is lower already, and in the idle I/O class, so its reads
// wait behind everyone else's.
func backgroundPriority() {
	tid := syscall.Gettid()
	if raw, err := syscall.Getpriority(syscall.PRIO_PROCESS, tid); err == nil && 20-raw < backgroundNice {
		_ = syscall.Setpriority(syscall.PRIO_PROCESS, tid, backgroundNice)
	}
	_, _, _ = syscall.Syscall(unix.SYS_IOPRIO_SET, ioprioWhoProcess, uintptr(tid), ioprioClassIdle<<ioprioClassShift)
}
