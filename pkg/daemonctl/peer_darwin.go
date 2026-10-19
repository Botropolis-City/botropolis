package daemonctl

import (
	"errors"

	"golang.org/x/sys/unix"
)

func peerPID(fd int) (int, error) {
	return unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
}

var errNoProc = errors.New("macOS has no /proc")

func cgroup(int) (string, error) { return "", errNoProc }

func args(int) ([]string, error) { return nil, errNoProc }
