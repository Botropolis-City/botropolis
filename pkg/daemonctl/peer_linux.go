package daemonctl

import (
	"bytes"
	"errors"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

func peerPID(fd int) (int, error) {
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return 0, err
	}
	return int(cred.Pid), nil
}

func cgroup(pid int) (string, error) {
	out, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cgroup")
	return string(out), err
}

// args is what a process was started with, after its own name.
func args(pid int) ([]string, error) {
	out, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return nil, err
	}
	fields := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(fields) == 0 || len(fields[0]) == 0 {
		return nil, errors.New("no command line")
	}
	var rest []string
	for _, f := range fields[1:] {
		rest = append(rest, string(f))
	}
	return rest, nil
}
