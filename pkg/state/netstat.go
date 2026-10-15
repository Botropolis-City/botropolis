package state

import (
	"bufio"
	"strings"
)

const (
	netstatConnField = 5
	netstatAddrField = 8
)

// NetstatConnected reads `netstat -f unix -n` as macOS prints it. A BSD
// kernel copies the listener's path onto every socket it accepts, so a
// line naming sock with a peer in Conn is someone attached; the listener
// itself names sock with Conn 0.
func NetstatConnected(output, sock string) bool {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) <= netstatAddrField || strings.Join(fields[netstatAddrField:], " ") != sock {
			continue
		}
		if fields[netstatConnField] != "0" {
			return true
		}
	}
	return false
}
