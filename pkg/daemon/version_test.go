package daemon_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Item 149: every response says which daemon sent it, and a client
// works across a release apart, saying so once, and refuses across a
// protocol apart.
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

	t.Run("when the daemon is another release of the same protocol", func(t *testing.T) {
		var heard []string
		restore := proto.OnNotice(func(n string) { heard = append(heard, n) })
		t.Cleanup(restore)
		d := newDaemon(t, idleHome(t, "interactive"))
		d.SetPeer(proto.Peer{Release: "0.0.1", Protocol: proto.Protocol})
		sock := serve(t, d)
		snapshot := func() (state.Snapshot, error) {
			client, err := proto.Dial(sock)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			return client.Snapshot()
		}
		first, err := snapshot()
		_, _ = snapshot()

		t.Run("it should answer", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should carry the notice on the snapshot", func(t *testing.T) {
			assert.Equal(t, "daemon is 0.0.1, this is dev: restart it to update", first.Notice)
		})

		t.Run("it should say it once across connections", func(t *testing.T) {
			assert.Equal(t, []string{"daemon is 0.0.1, this is dev: restart it to update"}, heard)
		})
	})

	t.Run("when the daemon speaks another protocol", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.SetPeer(proto.Peer{Release: "9.0.0", Protocol: proto.Protocol + 1})
		sock := serve(t, d)
		refusal := "this is dev and the daemon is 9.0.0: restart this"
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

				t.Run("it should refuse, saying what to restart", func(t *testing.T) {
					assert.EqualError(t, err, refusal)
				})
			})
		}

		t.Run("and the client subscribes", func(t *testing.T) {
			client, err := proto.Dial(sock)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			updates, err := client.Subscribe(time.Time{})
			require.NoError(t, err)
			_, delivered := <-updates

			t.Run("it should deliver nothing", func(t *testing.T) {
				assert.False(t, delivered)
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
