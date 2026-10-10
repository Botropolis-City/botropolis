package daemon_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(t *testing.T, d *daemon.Daemon) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "botropolis.sock")
	listener, err := net.Listen("unix", sock)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = d.Serve(ctx, listener)
	}()
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		<-done
	})
	return sock
}

func preToolUse(tool string) json.RawMessage {
	return json.RawMessage(`{"hook_event_name":"PreToolUse","session_id":"` + sid + `","cwd":"` + cwd + `","tool_name":"` + tool + `"}`)
}

func TestServe(t *testing.T) {
	t.Run("when a client asks for a snapshot", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		snapshot, err := client.Snapshot()
		require.NoError(t, err)

		t.Run("it should return the daemon's sessions", func(t *testing.T) {
			require.Len(t, snapshot.Sessions, 1)
			assert.Equal(t, "Fix the CI queue", snapshot.Sessions[0].Title)
		})
	})

	t.Run("when a hook sends a PreToolUse event", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		sock := serve(t, d)
		started := time.Now()
		require.NoError(t, proto.SendEvent(sock, preToolUse("Bash")))
		client, err := proto.Dial(sock)
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		snapshot, err := client.Snapshot()
		elapsed := time.Since(started)
		require.NoError(t, err)

		t.Run("it should show up in the next snapshot", func(t *testing.T) {
			assert.Equal(t, state.Working, snapshot.Sessions[0].State)
		})

		t.Run("it should carry the tool name", func(t *testing.T) {
			assert.Equal(t, "Bash", snapshot.Sessions[0].Tool)
		})

		t.Run("it should take well under a hundred milliseconds end to end", func(t *testing.T) {
			assert.Less(t, elapsed, 100*time.Millisecond)
		})
	})

	t.Run("when a client subscribes", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		updates, err := client.Subscribe(time.Time{})
		require.NoError(t, err)

		t.Run("it should receive the current snapshot first", func(t *testing.T) {
			select {
			case update := <-updates:
				assert.Equal(t, state.NeedsYou, update.Snapshot.Sessions[0].State)
			case <-time.After(time.Second):
				t.Fatal("no initial snapshot")
			}
		})

		t.Run("and an event arrives", func(t *testing.T) {
			d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Grep"}, now)

			t.Run("it should receive the update", func(t *testing.T) {
				select {
				case update := <-updates:
					assert.Equal(t, "Grep", update.Snapshot.Sessions[0].Tool)
				case <-time.After(time.Second):
					t.Fatal("no update")
				}
			})
		})
	})

	t.Run("when a client asks for events", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Grep"}, now)
		d.Apply(claude.HookEvent{Name: claude.HookStop, SessionID: sid}, now.Add(time.Minute))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()

		t.Run("and since is before the log", func(t *testing.T) {
			logged, err := client.Events(t0)
			require.NoError(t, err)

			t.Run("it should list the needs-you event", func(t *testing.T) {
				require.Len(t, logged, 1)
				assert.Equal(t, events.NeedsYou, logged[0].Kind)
			})
		})

		t.Run("and since is the log's own moment", func(t *testing.T) {
			logged, err := client.Events(now)
			require.NoError(t, err)

			t.Run("it should list nothing", func(t *testing.T) {
				assert.Empty(t, logged)
			})
		})
	})

	t.Run("when a client subscribes with a since", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Grep"}, now)
		d.Apply(claude.HookEvent{Name: claude.HookStop, SessionID: sid}, now.Add(time.Minute))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		updates, err := client.Subscribe(t0)
		require.NoError(t, err)

		t.Run("it should carry the backlog with the first snapshot", func(t *testing.T) {
			select {
			case update := <-updates:
				require.Len(t, update.Events, 1)
				assert.Equal(t, events.NeedsYou, update.Events[0].Kind)
				assert.Len(t, update.Snapshot.Sessions, 1)
			case <-time.After(time.Second):
				t.Fatal("no initial update")
			}
		})
	})

	t.Run("when a client sends an unknown op", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		conn, err := net.Dial("unix", serve(t, d))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		require.NoError(t, proto.Write(conn, proto.Request{Op: "dance"}))
		var response proto.Response
		require.NoError(t, proto.NewDecoder(conn).Decode(&response))

		t.Run("it should answer with an error", func(t *testing.T) {
			assert.Contains(t, response.Error, "dance")
		})
	})

	t.Run("when a hook sends a malformed event", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		sock := serve(t, d)
		err := proto.SendEvent(sock, json.RawMessage(`{"nope":true}`))

		t.Run("it should report the rejection", func(t *testing.T) {
			assert.Error(t, err)
		})

		t.Run("it should keep serving", func(t *testing.T) {
			client, err := proto.Dial(sock)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			_, err = client.Snapshot()
			assert.NoError(t, err)
		})
	})
}
