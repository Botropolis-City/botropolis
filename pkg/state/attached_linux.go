package state

import (
	"bufio"
	"os"
	"strings"
)

const (
	procNetUnix          = "/proc/net/unix"
	unixSocketConnecting = "02"
	unixSocketConnected  = "03"
)

func UnixSocketConnected(sock string) bool {
	f, err := os.Open(procNetUnix)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 || fields[7] != sock {
			continue
		}
		if fields[5] == unixSocketConnecting || fields[5] == unixSocketConnected {
			return true
		}
	}
	return false
}
