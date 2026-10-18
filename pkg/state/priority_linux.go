package state

import "syscall"

// lowerPriority drops the calling thread to the lowest CPU priority,
// and with it the I/O priority Linux derives from it.
func lowerPriority() {
	_ = syscall.Setpriority(syscall.PRIO_PROCESS, syscall.Gettid(), 19)
}
