package proc

import "golang.org/x/sys/unix"

// startTime is the kernel's p_starttime for pid, in microseconds since
// the epoch, from the same sysctl ps reads.
func startTime(pid int) (uint64, bool) {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || info.Proc.P_pid != int32(pid) {
		return 0, false
	}
	started := info.Proc.P_starttime
	if started.Sec <= 0 {
		return 0, false
	}
	return uint64(started.Sec)*1_000_000 + uint64(started.Usec), true
}
