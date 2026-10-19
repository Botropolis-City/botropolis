package daemon_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Items 149 and 152: every response says which daemon sent it, and a
// client refuses a daemon of any other release or protocol, saying
// which socket it reached, except to deliver a hook's event.
func TestServeVersions(t *testing.T) {
	t.Run("when a client asks for a snapshot", func(t *testing.T) {
		conn, err := net.Dial("unix", serve(t, newDaemon(t, idleHome(t, "interactive"))))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		require.NoError(t, proto.Write(conn, proto.Request{Op: proto.OpSnapshot, Peer: proto.Self()}))
		var response proto.Response
		require.NoError(t, proto.NewDecoder(conn).Decode(&response))

		t.Run("it should answer with its own release and protocol", func(t *testing.T) {
			assert.Equal(t, proto.Self(), response.Peer)
		})
	})

	for _, c := range []struct {
		scenario string
		peer     proto.Peer
		refusal  string
	}{
		{"when the daemon is another release of the same protocol", proto.Peer{Release: "0.0.1", Protocol: proto.Protocol}, "daemon is 0.0.1, this is dev"},
		{"when the daemon speaks another protocol", proto.Peer{Release: "9.0.0", Protocol: proto.Protocol + 1}, "daemon is 9.0.0, this is dev"},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			d := newDaemon(t, idleHome(t, "interactive"))
			d.SetPeer(c.peer)
			sock := serve(t, d)
			ops := []struct {
				name string
				call func(c *proto.Client) error
			}{
				{"snapshot", func(c *proto.Client) error { _, err := c.Snapshot(); return err }},
				{"events", func(c *proto.Client) error { _, err := c.Events(time.Time{}); return err }},
				{"find", func(c *proto.Client) error { _, err := c.Find(find.Query{Words: "ci"}); return err }},
				{"pack", func(c *proto.Client) error { return c.Pack(sid, true) }},
				{"close up", func(c *proto.Client) error { return c.CloseUp(nil, nil) }},
				{"forget", func(c *proto.Client) error { return c.Forget(sid) }},
			}
			for _, op := range ops {
				t.Run("and the client asks for "+op.name, func(t *testing.T) {
					client, err := proto.Dial(sock)
					require.NoError(t, err)
					defer func() { _ = client.Close() }()
					err = op.call(client)
					var mismatch *proto.Mismatch
					errors.As(err, &mismatch)

					t.Run("it should refuse, naming both", func(t *testing.T) {
						assert.EqualError(t, err, c.refusal)
					})

					t.Run("it should say which socket it reached", func(t *testing.T) {
						require.NotNil(t, mismatch)
						assert.Equal(t, sock, mismatch.Socket)
					})
				})
			}

			t.Run("and the client subscribes", func(t *testing.T) {
				client, err := proto.Dial(sock)
				require.NoError(t, err)
				defer func() { _ = client.Close() }()
				_, err = client.Subscribe(time.Time{})
				var mismatch *proto.Mismatch

				t.Run("it should refuse with the mismatch", func(t *testing.T) {
					assert.ErrorAs(t, err, &mismatch)
				})
			})

			t.Run("and a hook sends an event", func(t *testing.T) {
				err := proto.SendEvent(sock, preToolUse("Bash"))

				t.Run("it should still deliver it", func(t *testing.T) {
					assert.NoError(t, err)
				})

				t.Run("it should reach the daemon", func(t *testing.T) {
					assert.Equal(t, state.Working, d.Snapshot().Sessions[0].State)
				})
			})
		})
	}
}
