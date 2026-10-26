package control

import "golang.org/x/sys/unix"

const fdDir = "/proc/self/fd"

func closeRangeOnExec() bool {
	return unix.CloseRange(3, ^uint(0), unix.CLOSE_RANGE_CLOEXEC) == nil
}
