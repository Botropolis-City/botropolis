package app

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

type cancelOnWrite struct{ cancel context.CancelFunc }

func (w cancelOnWrite) Write(p []byte) (int, error) { w.cancel(); return len(p), nil }

// Item 336: the read-only subcommands under --home read that home's
// daemon, or the home itself, and never the default socket's.
func TestHomeSubcommandSocket(t *testing.T) {
	stage := func(t *testing.T, home bool, socket string) (*services, *config.Config, *fakeDaemon) {
		t.Setenv("XDG_RUNTIME_DIR", shortDir(t))
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		real := listenAt(t, proto.SocketPath())
		cfg := &config.Config{Home: t.TempDir(), HomeSet: home, Socket: proto.SocketPath()}
		if socket != "" {
			cfg.Socket, cfg.SocketSet = socket, true
		}
		return newServices(state.Probes{}), cfg, real
	}
	bounded := func(d time.Duration) (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), d)
	}
	subcommands := []struct {
		name string
		do   func(s *services, cfg *config.Config) error
	}{
		{"and events lists the event log", func(s *services, cfg *config.Config) error {
			return s.Events(cfg).Run(io.Discard, time.Now().Add(-time.Hour))
		}},
		{"and find searches", func(s *services, cfg *config.Config) error {
			return s.Find(cfg).Run(io.Discard, find.Query{Words: "kiln"})
		}},
		{"and bar follows the live feed", func(s *services, cfg *config.Config) error {
			ctx, cancel := bounded(3 * time.Second)
			defer cancel()
			return s.Bar(cfg).Watch(ctx, cancelOnWrite{cancel}, commands.BarText)
		}},
		{"and notify follows the live feed", func(s *services, cfg *config.Config) error {
			ctx, cancel := bounded(500 * time.Millisecond)
			defer cancel()
			return s.Notify(cfg).Run(ctx)
		}},
		{"and the TUI follows the live feed", func(s *services, cfg *config.Config) error {
			ctx, cancel := bounded(3 * time.Second)
			defer cancel()
			return s.feed(cfg).RunRefusing(ctx, func(state.Snapshot) { cancel() })
		}},
	}

	t.Run("when a subcommand runs under --home without --socket", func(t *testing.T) {
		for _, sub := range subcommands {
			t.Run(sub.name, func(t *testing.T) {
				s, cfg, real := stage(t, true, "")
				_ = sub.do(s, cfg)

				t.Run("it should never reach the default daemon", func(t *testing.T) {
					assert.Empty(t, real.received())
				})
			})
		}

		t.Run("and events is asked for the log", func(t *testing.T) {
			s, cfg, _ := stage(t, true, "")
			err := s.Events(cfg).Run(io.Discard, time.Now())

			t.Run("it should refuse, saying the home has no daemon of its own", func(t *testing.T) {
				assert.ErrorContains(t, err, "--socket")
			})
		})
	})

	t.Run("when a subcommand runs under --home with --socket", func(t *testing.T) {
		for _, sub := range subcommands {
			t.Run(sub.name, func(t *testing.T) {
				own := filepath.Join(shortDir(t), "own.sock")
				s, cfg, real := stage(t, true, own)
				mine := listenAt(t, own)
				_ = sub.do(s, cfg)

				t.Run("it should never reach the default daemon", func(t *testing.T) {
					assert.Empty(t, real.received())
				})
				t.Run("it should ask that home's daemon", func(t *testing.T) {
					require.NotEmpty(t, mine.received())
				})
			})
		}
	})
}
