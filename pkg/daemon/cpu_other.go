//go:build !linux

package daemon

import "time"

var started = time.Now()

// threadCPU is, where no thread's CPU time can be read, the wall clock,
// which only ever counts more than the thread used, so the pace errs
// long.
func threadCPU() time.Duration {
	return time.Since(started)
}
