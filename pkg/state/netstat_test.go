package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

const netstatUnix = `Active LOCAL (UNIX) domain sockets
Address          Type   Recv-Q Send-Q            Inode             Conn             Refs          Nextref Addr
a1b2c3d4e5f60001 stream      0      0                0 a1b2c3d4e5f60002                0                0 /tmp/cc-daemon-501/ffe95242/pty/0898d7e4.sock
a1b2c3d4e5f60002 stream      0      0                0 a1b2c3d4e5f60001                0                0
a1b2c3d4e5f60003 stream      0      0 a1b2c3d4e5f6f003                0                0                0 /tmp/cc-daemon-501/ffe95242/pty/0898d7e4.sock
a1b2c3d4e5f60004 stream      0      0 a1b2c3d4e5f6f004                0                0                0 /tmp/cc-daemon-501/ffe95242/pty/5d2e11a0.sock
a1b2c3d4e5f60005 dgram       0      0                0 a1b2c3d4e5f60006                0                0 /var/run/syslog
`

func TestNetstatConnected(t *testing.T) {
	cases := []struct {
		scenario string
		output   string
		sock     string
		want     bool
	}{
		{"when an accepted connection carries the socket's path", netstatUnix, "/tmp/cc-daemon-501/ffe95242/pty/0898d7e4.sock", true},
		{"when only the listener carries the socket's path", netstatUnix, "/tmp/cc-daemon-501/ffe95242/pty/5d2e11a0.sock", false},
		{"when no line carries the socket's path", netstatUnix, "/tmp/cc-daemon-501/ffe95242/pty/ffffffff.sock", false},
		{"when the socket's path is a prefix of another", netstatUnix, "/tmp/cc-daemon-501/ffe95242/pty/0898d7e4", false},
		{"when the output is empty", "", "/tmp/cc-daemon-501/ffe95242/pty/0898d7e4.sock", false},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			t.Run("it should report whether anyone is connected", func(t *testing.T) {
				assert.Equal(t, c.want, state.NetstatConnected(c.output, c.sock))
			})
		})
	}
}
