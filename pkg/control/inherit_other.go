//go:build !linux

package control

const fdDir = "/dev/fd"

func closeRangeOnExec() bool { return false }
