package daemon_test

import (
	"bytes"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Item 201: a subscription is sent the whole view once, then what
// changed since what that connection was last sent.
func TestStreamSendsDeltas(t *testing.T) {
	type wire struct {
		conn    net.Conn
		decoder *json.Decoder
		mirror  proto.Mirror
	}
	attach := func(t *testing.T, sock string) *wire {
		t.Helper()
		conn, err := net.Dial("unix", sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.NoError(t, proto.Write(conn, proto.Request{Op: proto.OpSubscribe, Peer: proto.Self()}))
		return &wire{conn: conn, decoder: proto.NewDecoder(conn)}
	}
	next := func(t *testing.T, w *wire) (proto.Response, state.Snapshot) {
		t.Helper()
		require.NoError(t, w.conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		var r proto.Response
		require.NoError(t, w.decoder.Decode(&r))
		held, err := w.mirror.Take(r)
		require.NoError(t, err)
		return r, held
	}
	view := func(t *testing.T, d *daemon.Daemon) state.Snapshot {
		t.Helper()
		s := d.Snapshot()
		var buf bytes.Buffer
		require.NoError(t, proto.Write(&buf, proto.Response{Snapshot: &s}))
		var r proto.Response
		require.NoError(t, proto.NewDecoder(&buf).Decode(&r))
		return *r.Snapshot
	}

	t.Run("when a subscriber is attached and one session's state changes", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		w := attach(t, serve(t, d))
		next(t, w)
		d.Apply(event(claude.HookPreToolUse, "Bash"), time.Now())
		r, held := next(t, w)

		t.Run("it should be sent a delta, not a snapshot", func(t *testing.T) {
			assert.True(t, r.Delta != nil && r.Snapshot == nil)
		})

		t.Run("its client should hold the daemon's view", func(t *testing.T) {
			assert.Equal(t, view(t, d), held)
		})
	})

	t.Run("when the subscriber falls behind and updates are folded together", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		w := attach(t, serve(t, d))
		next(t, w)
		tools := []string{"Bash", "Read", "Edit", "Grep", "Write", "Glob", "Task", "WebFetch"}
		for i := range 41 {
			d.Apply(event(claude.HookPreToolUse, tools[i%len(tools)]), time.Now())
		}
		want := view(t, d)
		var held state.Snapshot
		for len(held.Sessions) == 0 || held.Sessions[0].LastActivity.Before(want.Sessions[0].LastActivity) {
			_, held = next(t, w)
		}

		t.Run("its client should hold the newest view", func(t *testing.T) {
			assert.Equal(t, want, held)
		})
	})
}
