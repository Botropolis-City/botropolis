package app

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/daemonctl"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/statedir"
)

type fakeDaemon struct {
	mu  sync.Mutex
	ops []string
}

func (d *fakeDaemon) received() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.ops...)
}

func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "b334")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func listenAt(t *testing.T, path string) *fakeDaemon {
	t.Helper()
	requireBindable(t, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	l, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	d := &fakeDaemon{}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			var req proto.Request
			_ = proto.NewDecoder(conn).Decode(&req)
			d.mu.Lock()
			d.ops = append(d.ops, req.Op)
			d.mu.Unlock()
			_ = conn.Close()
		}
	}()
	return d
}

// Item 334: a city under --home talks only to a daemon for that home,
// and never to the default socket's, which keeps the real ledger.
func TestCityActorSocket(t *testing.T) {
	const session = "33400000-0000-0000-0000-000000000000"

	stage := func(t *testing.T, home bool, socket string) (*liveActor, *fakeDaemon) {
		stageDefaultSocket(t)
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		cleanup, err := statedir.MakePrivate("")
		require.NoError(t, err)
		t.Cleanup(func() { _ = cleanup() })
		real := listenAt(t, proto.SocketPath())
		cfg := &config.Config{Home: t.TempDir(), HomeSet: home, Socket: proto.SocketPath()}
		if socket != "" {
			cfg.Socket, cfg.SocketSet = socket, true
		}
		return cityActor(cfg), real
	}

	t.Run("when the city runs under --home without --socket", func(t *testing.T) {
		actions := []struct {
			name string
			do   func(a *liveActor) error
		}{
			{"and a session is packed", func(a *liveActor) error { return a.pack(session, true) }},
			{"and a close-up is recorded", func(a *liveActor) error { return a.closeUp([]string{session}, nil) }},
		}
		for _, action := range actions {
			t.Run(action.name, func(t *testing.T) {
				actor, real := stage(t, true, "")
				err := action.do(actor)

				t.Run("it should refuse, saying the home has no daemon of its own", func(t *testing.T) {
					assert.ErrorContains(t, err, "--socket")
				})
				t.Run("it should never reach the default daemon", func(t *testing.T) {
					assert.Empty(t, real.received())
				})
			})
		}

		t.Run("and a session is demolished", func(t *testing.T) {
			actor, real := stage(t, true, "")
			_ = actor.demolished(session)

			t.Run("it should never reach the default daemon", func(t *testing.T) {
				assert.Empty(t, real.received())
			})
		})
	})

	t.Run("when the city runs under --home with --socket", func(t *testing.T) {
		own := filepath.Join(shortDir(t), "own.sock")
		actor, _ := stage(t, true, own)
		mine := listenAt(t, own)
		_ = actor.pack(session, true)

		t.Run("it should pack through that home's daemon", func(t *testing.T) {
			assert.Equal(t, []string{proto.OpPack}, mine.received())
		})
	})

	t.Run("when the city runs as the person's own", func(t *testing.T) {
		actor, real := stage(t, false, "")
		_ = actor.pack(session, true)

		t.Run("it should pack through the default daemon", func(t *testing.T) {
			assert.Equal(t, []string{proto.OpPack}, real.received())
		})
	})
}

type stagedSource struct{}

func (stagedSource) Snapshot(bool) (state.Snapshot, error) { return state.Snapshot{}, nil }

// Item 334, slice 2: what the city reads, and the restart it offers, go
// to that home's daemon too.
func TestCityDaemon(t *testing.T) {
	stage := func(t *testing.T, home bool, socket string) (cityDaemon, *fakeDaemon) {
		stageDefaultSocket(t)
		real := listenAt(t, proto.SocketPath())
		cfg := &config.Config{Home: t.TempDir(), HomeSet: home, Socket: proto.SocketPath()}
		if socket != "" {
			cfg.Socket, cfg.SocketSet = socket, true
		}
		d := newCityDaemon(cfg, stagedSource{})
		d.system = daemonctl.System{PeerPID: func(sock string) (int, error) {
			real.mu.Lock()
			defer real.mu.Unlock()
			real.ops = append(real.ops, "restart "+sock)
			return 0, errors.New("a test restarts nothing")
		}}
		return d, real
	}
	follow := func(d cityDaemon) {
		ctx, cancel := context.WithCancel(context.Background())
		feed := d.feed()
		feed.Run(ctx, func(state.Snapshot) { cancel() })
	}
	reads := []struct {
		name string
		op   string
		do   func(d cityDaemon)
	}{
		{"and it follows the sessions", proto.OpSubscribe, follow},
		{"and it reads the loading meter", proto.OpLoading, func(d cityDaemon) { _, _ = d.loading() }},
		{"and it searches", proto.OpFind, func(d cityDaemon) { _, _ = d.search(find.Query{Words: "kiln"}) }},
	}

	t.Run("when the city runs under --home without --socket", func(t *testing.T) {
		for _, read := range reads {
			t.Run(read.name, func(t *testing.T) {
				d, real := stage(t, true, "")
				read.do(d)

				t.Run("it should never reach the default daemon", func(t *testing.T) {
					assert.Empty(t, real.received())
				})
			})
		}

		t.Run("and the daemon is restarted from its settings", func(t *testing.T) {
			d, real := stage(t, true, "")
			err := d.restart(context.Background())

			t.Run("it should refuse, saying the home has no daemon of its own", func(t *testing.T) {
				assert.ErrorContains(t, err, "--socket")
			})
			t.Run("it should never reach the default daemon", func(t *testing.T) {
				assert.Empty(t, real.received())
			})
		})
	})

	t.Run("when the city runs under --home with --socket", func(t *testing.T) {
		for _, read := range reads {
			t.Run(read.name, func(t *testing.T) {
				own := filepath.Join(shortDir(t), "own.sock")
				d, _ := stage(t, true, own)
				mine := listenAt(t, own)
				read.do(d)

				t.Run("it should ask that home's daemon", func(t *testing.T) {
					assert.Contains(t, mine.received(), read.op)
				})
			})
		}
	})
}
