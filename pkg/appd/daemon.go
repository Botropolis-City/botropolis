package appd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/harness"
	"botropolis.city/botropolis/pkg/harness/codex"
	"botropolis.city/botropolis/pkg/state"
)

const maxSocketPath = 107

func DaemonModule(cfg *config.Config, out io.Writer) fx.Option {
	return fx.Module("botropolisd",
		fx.Supply(cfg),
		fx.Supply(fx.Annotate(out, fx.As(new(io.Writer)))),
		fx.Provide(NewProbes, newDaemon, newListener),
		fx.Invoke(runDaemon),
	)
}

// DefaultMemoryLimit caps the Go heap so the idle daemon stays near the
// 20 MB target design/what-it-costs.md sets; GOMEMLIMIT overrides it.
const DefaultMemoryLimit = 16 << 20

const (
	startTimeout = 10 * time.Second
	stopTimeout  = 5 * time.Second
)

func RunDaemon(ctx context.Context, cfg *config.Config, out io.Writer) error {
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(DefaultMemoryLimit)
	}
	app := fx.New(
		DaemonModule(cfg, out),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	)
	if err := app.Err(); err != nil {
		return err
	}
	// Startup runs to completion on its own clock: a stop signal that
	// lands while the socket is being brought up is an orderly shutdown
	// once it is up, not a failed start, so `systemctl stop` during
	// startup exits zero and leaves no socket behind.
	startCtx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	if err := app.Start(startCtx); err != nil {
		return err
	}
	var exit error
	select {
	case <-ctx.Done():
	case sig := <-app.Wait():
		if sig.ExitCode != 0 {
			exit = fmt.Errorf("daemon stopped with exit code %d", sig.ExitCode)
		}
	}
	stopCtx, cancelStop := context.WithTimeout(context.Background(), stopTimeout)
	defer cancelStop()
	if err := app.Stop(stopCtx); err != nil && exit == nil {
		exit = err
	}
	return exit
}

func NewProbes() state.Probes {
	return state.Probes{Alive: state.ProcessAlive, Attached: state.UnixSocketConnected}
}

func newDaemon(cfg *config.Config, probes state.Probes) *daemon.Daemon {
	d := daemon.NewWith(harnesses(cfg, probes, ledgerPath(cfg)), time.Now)
	d.SetIndex(find.New(cfg.Home))
	return d
}

// ledgerPath is where the daemon keeps its ledger of sessions shown, and
// "" for a home it was pointed at: the ledger is the default home's
// (item 86).
func ledgerPath(cfg *config.Config) string {
	if cfg.HomeSet {
		return ""
	}
	return state.LedgerPath()
}

// Harnesses loads directly, without a daemon, and keeps no ledger: only
// the daemon writes it.
func Harnesses(cfg *config.Config, probes state.Probes) *harness.Multi {
	return harnesses(cfg, probes, "")
}

func harnesses(cfg *config.Config, probes state.Probes, ledger string) *harness.Multi {
	claudeLoader := harness.NewClaude(state.NewLoader(cfg.Home, probes).WithGroups(group.Path()).WithLedger(ledger))
	harnesses := []harness.Snapshotter{claudeLoader}
	if info, err := os.Stat(filepath.Join(cfg.CodexHome, "sessions")); err == nil && info.IsDir() {
		harnesses = append(harnesses, codex.NewLoader(cfg.CodexHome))
	}
	return harness.NewMulti(harnesses...)
}

func newListener(cfg *config.Config) (net.Listener, error) {
	sock := cfg.Socket
	if len(sock) > maxSocketPath {
		return nil, fmt.Errorf("socket path is %d bytes; unix sockets allow at most %d: %s", len(sock), maxSocketPath, sock)
	}
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return nil, err
	}
	if conn, err := net.DialTimeout("unix", sock, 200*time.Millisecond); err == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("another daemon is serving %s", sock)
	}
	if err := os.Remove(sock); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", sock)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	return listener, nil
}

func runDaemon(lc fx.Lifecycle, shutdowner fx.Shutdowner, d *daemon.Daemon, listener net.Listener, cfg *config.Config, out io.Writer) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if err := d.Rescan(); err != nil {
				return err
			}
			fmt.Fprintf(out, "botropolisd: serving %s for %s\n", cfg.Socket, cfg.Home)
			errs := make(chan error, 2)
			go func() { errs <- d.Watch(ctx) }()
			go func() { errs <- d.Serve(ctx, listener) }()
			go func() {
				defer close(done)
				select {
				case <-ctx.Done():
				case err := <-errs:
					if err != nil {
						_ = shutdowner.Shutdown(fx.ExitCode(1))
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			_ = listener.Close()
			<-done
			_ = os.Remove(cfg.Socket)
			return nil
		},
	})
}
