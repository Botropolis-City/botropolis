package state

import "os/exec"

func UnixSocketConnected(sock string) bool {
	out, err := exec.Command("/usr/sbin/netstat", "-f", "unix", "-n").Output()
	if err != nil {
		return false
	}
	return NetstatConnected(string(out), sock)
}
