package acceptance_test

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// oldDaemon answers every request on a socket as a daemon of another
// release would, with an empty snapshot.
func oldDaemon(t *testing.T, peer proto.Peer) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "d.sock")
	listener, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				decoder := proto.NewDecoder(conn)
				for {
					var request proto.Request
					if decoder.Decode(&request) != nil {
						return
					}
					if proto.Write(conn, proto.Response{Peer: peer, Snapshot: &state.Snapshot{}}) != nil {
						return
					}
				}
			}()
		}
	}()
	return sock
}

// Item 149: after an upgrade the old daemon keeps running until it is
// restarted; a command says so once, on stderr, and works.
func TestDaemonOfAnotherRelease(t *testing.T) {
	t.Run("when status --json reaches a daemon of another release", func(t *testing.T) {
		sock := oldDaemon(t, proto.Peer{Release: "0.0.1", Protocol: proto.Protocol})
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(botropolis, "status", "--json", "--socket", sock)
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()

		t.Run("it should succeed", func(t *testing.T) {
			assert.NoError(t, err, stderr.String())
		})

		t.Run("it should say so on stderr once", func(t *testing.T) {
			assert.Equal(t, 1, strings.Count(stderr.String(), "botropolis: daemon is 0.0.1, this is dev: restart it to update\n"), stderr.String())
		})

		t.Run("it should keep stdout parseable", func(t *testing.T) {
			assert.True(t, json.Valid(stdout.Bytes()), stdout.String())
		})
	})
}
