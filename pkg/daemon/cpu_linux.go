package daemon

import (
	"time"

	"golang.org/x/sys/unix"
)

// threadCPU is the CPU time the calling thread has used.
func threadCPU() time.Duration {
	var u unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_THREAD, &u); err != nil {
		return 0
	}
	return time.Duration(u.Utime.Nano() + u.Stime.Nano())
}
