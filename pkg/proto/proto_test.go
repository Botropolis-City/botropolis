package proto_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocketPath(t *testing.T) {
	t.Run("when XDG_RUNTIME_DIR is set", func(t *testing.T) {
		t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

		t.Run("it should live under it", func(t *testing.T) {
			assert.Equal(t, "/run/user/1000/botropolis/botropolis.sock", proto.SocketPath())
		})
	})

	t.Run("when XDG_RUNTIME_DIR is not set", func(t *testing.T) {
		t.Setenv("XDG_RUNTIME_DIR", "")

		t.Run("it should fall back to a per-user directory under the temp dir", func(t *testing.T) {
			assert.Equal(t, filepath.Join(os.TempDir(), "botropolis-"+strconv.Itoa(os.Getuid()), "botropolis.sock"), proto.SocketPath())
		})
	})
}

func TestCodec(t *testing.T) {
	t.Run("when a snapshot request is written and read back", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, proto.Write(&buf, proto.Request{Op: proto.OpSnapshot}))
		var got proto.Request
		require.NoError(t, proto.NewDecoder(&buf).Decode(&got))

		t.Run("it should keep the op", func(t *testing.T) {
			assert.Equal(t, proto.OpSnapshot, got.Op)
		})

		t.Run("it should end the frame with a newline", func(t *testing.T) {
			var frame bytes.Buffer
			require.NoError(t, proto.Write(&frame, proto.Request{Op: proto.OpSnapshot}))
			assert.True(t, bytes.HasSuffix(frame.Bytes(), []byte("\n")))
		})
	})

	t.Run("when an event request carries a raw hook payload", func(t *testing.T) {
		var buf bytes.Buffer
		payload := json.RawMessage(`{"hook_event_name":"PreToolUse","session_id":"abc","tool_name":"Bash"}`)
		require.NoError(t, proto.Write(&buf, proto.Request{Op: proto.OpEvent, Event: payload}))
		var got proto.Request
		require.NoError(t, proto.NewDecoder(&buf).Decode(&got))

		t.Run("it should preserve the payload byte for byte", func(t *testing.T) {
			assert.JSONEq(t, string(payload), string(got.Event))
		})
	})

	t.Run("when a response carries a snapshot", func(t *testing.T) {
		var buf bytes.Buffer
		snapshot := state.Snapshot{Sessions: []state.Session{{ID: "abc", State: state.Working, ContextPercent: 12.5}}}
		require.NoError(t, proto.Write(&buf, proto.Response{Snapshot: &snapshot}))
		var got proto.Response
		require.NoError(t, proto.NewDecoder(&buf).Decode(&got))

		t.Run("it should round-trip the sessions", func(t *testing.T) {
			assert.Equal(t, snapshot.Sessions, got.Snapshot.Sessions)
		})
	})

	t.Run("when an events request is written and read back", func(t *testing.T) {
		var buf bytes.Buffer
		since := time.Date(2026, time.September, 18, 20, 0, 0, 0, time.UTC)
		require.NoError(t, proto.Write(&buf, proto.Request{Op: proto.OpEvents, Since: since}))
		var got proto.Request
		require.NoError(t, proto.NewDecoder(&buf).Decode(&got))

		t.Run("it should carry since", func(t *testing.T) {
			assert.True(t, got.Since.Equal(since))
		})
	})

	t.Run("when a subscribe request has no since", func(t *testing.T) {
		var frame bytes.Buffer
		require.NoError(t, proto.Write(&frame, proto.Request{Op: proto.OpSubscribe}))

		t.Run("it should leave since off the wire", func(t *testing.T) {
			assert.NotContains(t, frame.String(), "since")
		})
	})

	t.Run("when a response carries events", func(t *testing.T) {
		var buf bytes.Buffer
		at := time.Date(2026, time.September, 18, 20, 0, 0, 0, time.UTC)
		logged := []events.Event{{At: at, Kind: events.NeedsYou, SessionID: "abc", Title: "Fix the CI queue"}}
		require.NoError(t, proto.Write(&buf, proto.Response{Events: logged}))
		var got proto.Response
		require.NoError(t, proto.NewDecoder(&buf).Decode(&got))

		t.Run("it should round-trip the events", func(t *testing.T) {
			assert.Equal(t, logged, got.Events)
		})
	})

	t.Run("when the reader hits end of input", func(t *testing.T) {
		var got proto.Request
		err := proto.NewDecoder(&bytes.Buffer{}).Decode(&got)

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
