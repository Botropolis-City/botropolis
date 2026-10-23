package proto_test

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Item 201: a subscription is streamed deltas, which the client applies
// so that what it hands on is still the whole snapshot.
func TestSubscribeWithDeltas(t *testing.T) {
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	view := func(working ...string) state.Snapshot {
		s := state.Snapshot{At: at}
		for _, id := range []string{"a", "b", "c"} {
			st := state.Resting
			for _, w := range working {
				if w == id {
					st = state.Working
				}
			}
			s.Sessions = append(s.Sessions, state.Session{ID: id, Title: "Session " + id, State: st, StartedAt: at})
		}
		return s
	}
	whole := func(t *testing.T, s state.Snapshot) state.Snapshot {
		t.Helper()
		var buf bytes.Buffer
		require.NoError(t, proto.Write(&buf, proto.Response{Snapshot: &s}))
		var got proto.Response
		require.NoError(t, proto.NewDecoder(&buf).Decode(&got))
		return *got.Snapshot
	}
	// daemon answers one subscription with the responses given, as this
	// build, and then holds the connection open until the test ends.
	daemon := func(t *testing.T, peer proto.Peer, responses ...proto.Response) string {
		t.Helper()
		dir, err := os.MkdirTemp("", "bt")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		sock := filepath.Join(dir, "d.sock")
		listener, err := net.Listen("unix", sock)
		require.NoError(t, err)
		done := make(chan struct{})
		t.Cleanup(func() { close(done); _ = listener.Close() })
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			var request proto.Request
			if err := proto.NewDecoder(conn).Decode(&request); err != nil {
				return
			}
			for _, r := range responses {
				r.Peer = peer
				if err := proto.Write(conn, r); err != nil {
					return
				}
			}
			<-done
		}()
		return sock
	}
	stream := func(views ...state.Snapshot) []proto.Response {
		sender := proto.NewSender()
		var out []proto.Response
		for _, v := range views {
			r, _ := sender.Next(v)
			out = append(out, r)
		}
		return out
	}
	subscribe := func(t *testing.T, sock string) (<-chan proto.Update, error) {
		t.Helper()
		client, err := proto.Dial(sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		return client.Subscribe(time.Time{})
	}
	take := func(t *testing.T, updates <-chan proto.Update, n int) []state.Snapshot {
		t.Helper()
		var got []state.Snapshot
		for range n {
			select {
			case u, ok := <-updates:
				require.True(t, ok, "the subscription ended")
				got = append(got, u.Snapshot)
			case <-time.After(5 * time.Second):
				t.Fatal("no update")
			}
		}
		return got
	}

	t.Run("when a full snapshot then deltas arrive", func(t *testing.T) {
		views := []state.Snapshot{view(), view("a"), view("a", "c"), view("c")}
		responses := stream(views...)
		require.NotNil(t, responses[1].Delta)
		var sent bytes.Buffer
		for _, r := range responses {
			r.Peer = proto.Self()
			require.NoError(t, proto.Write(&sent, r))
		}
		reading, received := proto.Reading(), proto.Received()
		updates, err := subscribe(t, daemon(t, proto.Self(), responses...))
		require.NoError(t, err)
		got := take(t, updates, len(views))

		t.Run("it should count the time spent reading them", func(t *testing.T) {
			assert.Greater(t, proto.Reading().CPU, reading.CPU)
		})

		t.Run("it should count the bytes it read from the daemon", func(t *testing.T) {
			assert.Equal(t, int64(sent.Len()), proto.Received()-received)
		})

		t.Run("it should hand on whole snapshots equal to the daemon's views", func(t *testing.T) {
			var want []state.Snapshot
			for _, v := range views {
				want = append(want, whole(t, v))
			}
			assert.Equal(t, want, got)
		})
	})

	t.Run("when a full snapshot arrives mid-stream", func(t *testing.T) {
		responses := stream(view(), view("a"))
		other := view("b")
		other.Sessions = other.Sessions[1:]
		responses = append(responses, proto.Response{Seq: 9, Snapshot: &other})
		updates, err := subscribe(t, daemon(t, proto.Self(), responses...))
		require.NoError(t, err)
		got := take(t, updates, 3)

		t.Run("it should take it in place of what it held", func(t *testing.T) {
			assert.Equal(t, whole(t, other), got[2])
		})
	})

	t.Run("when a delta arrives on a base it does not hold", func(t *testing.T) {
		responses := stream(view(), view("a"), view("b"))
		updates, err := subscribe(t, daemon(t, proto.Self(), responses[0], responses[2]))
		require.NoError(t, err)
		take(t, updates, 1)
		ended := false
		select {
		case _, ok := <-updates:
			ended = !ok
		case <-time.After(5 * time.Second):
		}

		t.Run("it should end the subscription", func(t *testing.T) {
			assert.True(t, ended)
		})
	})

	t.Run("when the daemon speaks protocol 1", func(t *testing.T) {
		old := proto.Self()
		old.Protocol = 1
		first := view()
		_, err := subscribe(t, daemon(t, old, proto.Response{Snapshot: &first}))
		var mismatch *proto.Mismatch

		t.Run("it should be refused as out of sync, for item 152's dialog", func(t *testing.T) {
			assert.ErrorAs(t, err, &mismatch)
		})
	})
}
