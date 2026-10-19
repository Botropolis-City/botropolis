package commands_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

var olderDaemon = proto.Peer{Release: "0.0.1", Protocol: proto.Protocol}

// directSource records whether anything scanned directly.
type directSource struct {
	mu      sync.Mutex
	scanned bool
}

func (s *directSource) Snapshot(bool) (state.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanned = true
	return state.Snapshot{}, nil
}

func (s *directSource) didScan() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanned
}

// Item 152: a daemon of another release stops every client rather than
// being worked around.
func TestFeedOutOfSync(t *testing.T) {
	t.Run("when the daemon is another release", func(t *testing.T) {
		d, sock := servedDaemon(t)
		d.SetPeer(olderDaemon)
		source := &directSource{}
		mismatches := make(chan *proto.Mismatch, 16)
		offered := make(chan state.Snapshot, 16)
		feed := commands.Feed{Socket: sock, Source: source, Poll: time.Millisecond, Retry: 10 * time.Millisecond,
			Mismatch: func(m *proto.Mismatch) { mismatches <- m }}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go feed.Run(ctx, func(s state.Snapshot) { offered <- s })
		var first *proto.Mismatch
		select {
		case first = <-mismatches:
		case <-time.After(2 * time.Second):
		}

		t.Run("it should report the mismatch", func(t *testing.T) {
			assert.EqualError(t, first, "the Botropolis city and its daemon must be running the same version (daemon 0.0.1, city dev)")
		})

		t.Run("it should offer no snapshot", func(t *testing.T) {
			assert.Empty(t, offered)
		})

		t.Run("it should not scan directly", func(t *testing.T) {
			assert.False(t, source.didScan())
		})

		t.Run("and the daemon comes back at this release", func(t *testing.T) {
			d.SetPeer(proto.Self())

			t.Run("it should offer the snapshot", func(t *testing.T) {
				select {
				case s := <-offered:
					assert.Len(t, s.Sessions, 1)
				case <-time.After(2 * time.Second):
					t.Fatal("no snapshot offered")
				}
			})
		})
	})

	t.Run("when a refusing feed meets another release", func(t *testing.T) {
		d, sock := servedDaemon(t)
		d.SetPeer(olderDaemon)
		feed := commands.Feed{Socket: sock, Source: &directSource{}, Poll: time.Millisecond, Retry: 10 * time.Millisecond}
		done := make(chan error, 1)
		go func() { done <- feed.RunRefusing(context.Background(), func(state.Snapshot) {}) }()

		t.Run("it should return the mismatch", func(t *testing.T) {
			select {
			case err := <-done:
				var mismatch *proto.Mismatch
				assert.ErrorAs(t, err, &mismatch)
			case <-time.After(2 * time.Second):
				t.Fatal("the feed did not stop")
			}
		})
	})

	t.Run("when a refusing feed is cancelled", func(t *testing.T) {
		feed := commands.Feed{Socket: t.TempDir() + "/none.sock", Source: &directSource{}, Poll: time.Millisecond, Retry: time.Millisecond}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		t.Run("it should return no error", func(t *testing.T) {
			assert.NoError(t, feed.RunRefusing(ctx, func(state.Snapshot) {}))
		})
	})
}

func TestDaemonOrDirectOutOfSync(t *testing.T) {
	t.Run("when the daemon is another release", func(t *testing.T) {
		d, sock := servedDaemon(t)
		d.SetPeer(olderDaemon)
		scanned := false
		source := commands.DaemonOrDirect{Socket: sock, Now: time.Now,
			Direct: func(time.Time) (state.Snapshot, error) { scanned = true; return state.Snapshot{}, nil }}
		_, err := source.Snapshot(false)

		t.Run("it should refuse with the mismatch", func(t *testing.T) {
			var mismatch *proto.Mismatch
			assert.ErrorAs(t, err, &mismatch)
		})

		t.Run("it should not scan directly", func(t *testing.T) {
			assert.False(t, scanned)
		})
	})
}

func TestDoctorOutOfSync(t *testing.T) {
	t.Run("when the daemon is another release", func(t *testing.T) {
		mismatch := &proto.Mismatch{Daemon: olderDaemon, Ours: proto.Peer{Release: "0.2.3", Protocol: proto.Protocol}, Socket: "/run/none.sock"}
		d := doctorFor(t, true, map[string]string{"DISPLAY": ":0"}, []string{"claude"}, mismatch)
		d.RestartHint = func(m *proto.Mismatch) string {
			return `restart the daemon with "systemctl --user restart botropolisd"`
		}
		c := checkNamed(d.Run(), "daemon")

		t.Run("it should fail the daemon check", func(t *testing.T) {
			assert.Equal(t, commands.CheckFail, c.Status)
		})

		t.Run("it should name both releases", func(t *testing.T) {
			assert.Equal(t, "the Botropolis city and its daemon must be running the same version (daemon 0.0.1, city 0.2.3)", c.Detail)
		})

		t.Run("it should say how to restart it", func(t *testing.T) {
			assert.Equal(t, `restart the daemon with "systemctl --user restart botropolisd"`, c.Hint)
		})
	})

	t.Run("when the daemon cannot be reached", func(t *testing.T) {
		d := doctorFor(t, true, map[string]string{"DISPLAY": ":0"}, []string{"claude"}, errors.New("connection refused"))

		t.Run("it should still only warn", func(t *testing.T) {
			require.Equal(t, commands.CheckWarn, checkNamed(d.Run(), "daemon").Status)
		})
	})
}
