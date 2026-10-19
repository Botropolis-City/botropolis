package daemon_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
)

// answering serves one response to every request on a scratch socket.
func answering(t *testing.T, response proto.Response) string {
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
			var request proto.Request
			_ = proto.NewDecoder(conn).Decode(&request)
			_ = proto.Write(conn, response)
			_ = conn.Close()
		}
	}()
	return sock
}

// Item 152: asking a daemon only what it is, which a restart waits on.
func TestAsk(t *testing.T) {
	t.Run("when this daemon is asked", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.SetPeer(proto.Peer{Release: "0.0.1", Protocol: proto.Protocol})
		peer, err := proto.Ask(serve(t, d))
		require.NoError(t, err)

		t.Run("it should say its release and protocol", func(t *testing.T) {
			assert.Equal(t, proto.Peer{Release: "0.0.1", Protocol: proto.Protocol}, peer)
		})
	})

	t.Run("when a daemon that does not know the question is asked", func(t *testing.T) {
		older := proto.Peer{Release: "0.2.2", Protocol: 1}
		peer, err := proto.Ask(answering(t, proto.Response{Peer: older, Error: `unknown op "peer"`}))
		require.NoError(t, err)

		t.Run("it should still say what it is", func(t *testing.T) {
			assert.Equal(t, older, peer)
		})
	})

	t.Run("when a daemon from before the exchange is asked", func(t *testing.T) {
		peer, err := proto.Ask(answering(t, proto.Response{Error: `unknown op "peer"`}))
		require.NoError(t, err)

		t.Run("it should say nothing about itself", func(t *testing.T) {
			assert.Equal(t, proto.Peer{}, peer)
		})
	})

	t.Run("when nothing listens", func(t *testing.T) {
		_, err := proto.Ask(filepath.Join(t.TempDir(), "none.sock"))

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
