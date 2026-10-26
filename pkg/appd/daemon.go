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
	"sync"
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
	"botropolis.city/botropolis/pkg/statedir"
	"botropolis.city/botropolis/pkg/stats"
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

// DefaultMemoryLimit is the first launch's 256 MiB budget in
// performance/budgets.md, the most the daemon may ever hold, so the
// collector works hard only near it. Settled, the budget is 64 MiB spent
// on caches that save CPU, not on a tight heap: a 16 MiB cap kept the
// collector running continuously (item 157). GOMEMLIMIT overrides it.
const DefaultMemoryLimit = 256 << 20

// memoryLimit is the limit RunDaemon sets, or -1, which changes nothing,
// when GOMEMLIMIT has already set it.
func memoryLimit(env string) int64 {
	if env != "" {
		return -1
	}
	return DefaultMemoryLimit
}

const (
	startTimeout = 10 * time.Second
	stopTimeout  = 5 * time.Second
)

func RunDaemon(ctx context.Context, cfg *config.Config, out io.Writer) error {
	debug.SetMemoryLimit(memoryLimit(os.Getenv("GOMEMLIMIT")))
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
	var d *daemon.Daemon
	// Each pass of the whole read ends in a rescan, so the storeys it
	// measured reach the map (item 109).
	rescan := func() { d.RequestRescan() }
	d = daemon.NewWith(harnesses(cfg, probes, ledgerPath(cfg), rescan), time.Now)
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

// GroupsPath is where the groups and their corrections are kept, and
// "" for a home the run was pointed at: groups are the default home's,
// so a fixture or a test's home never reads or writes them (item 87).
// A private run keeps its own, in its private state (item 111).
func GroupsPath(cfg *config.Config) string {
	if cfg.HomeSet && !statedir.IsPrivate() {
		return ""
	}
	return group.Path()
}

// Speed is how fast a direct load reads the history in the background.
type Speed int

const (
	// Paced keeps the background read to half a core (item 159).
	Paced Speed = iota
	// FullSpeed reads for the whole clock, for a capture, which waits on
	// the read and leaves nothing else waiting (item 240).
	FullSpeed
)

// Harnesses loads directly, without a daemon, and keeps no ledger: only
// the daemon writes the person's own. A private run keeps the one in its
// private state, seeded from the home it was pointed at (item 111).
func Harnesses(cfg *config.Config, probes state.Probes, speed Speed) *harness.Multi {
	ledger := ""
	if statedir.IsPrivate() {
		ledger = state.LedgerPath()
	}
	// The city runs this way when there is no daemon, and it loads again
	// every few seconds, so it reads stopped sessions whole in the
	// background too, and the next load picks up what a pass found
	// (item 109).
	return harnessesAt(cfg, probes, ledger, func() {}, speed)
}

// harnesses reads every session; wholeRead, when not nil, has the Claude
// loader read stopped transcripts whole in the background for their
// heft and calls it after each pass. Only the daemon does that.
func harnesses(cfg *config.Config, probes state.Probes, ledger string, wholeRead func()) *harness.Multi {
	return harnessesAt(cfg, probes, ledger, wholeRead, Paced)
}

func harnessesAt(cfg *config.Config, probes state.Probes, ledger string, wholeRead func(), speed Speed) *harness.Multi {
	loader := state.NewLoader(cfg.Home, probes).WithGroups(GroupsPath(cfg)).WithLedger(ledger).WithCloseUp(closeUpPath(ledger))
	if wholeRead != nil {
		if src, _ := statsSource(cfg, os.Stderr); src != nil {
			loader = loader.WithStats(src, wholeRead)
		} else {
			loader = loader.WithWholeRead(wholeRead)
		}
	}
	if speed == FullSpeed {
		loader = loader.WithFullSpeed()
	}
	claudeLoader := harness.NewClaude(loader)
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
		// Starting only begins serving: reading the home takes as long as
		// the home is large, so it happens after Start, while requests wait
		// for it (item 143).
		OnStart: func(context.Context) error {
			d.HoldUntilLoaded()
			errs := make(chan error, 3)
			go func() { errs <- d.Serve(ctx, listener) }()
			go func() { errs <- load(ctx, d, errs) }()
			go func() {
				defer close(done)
				for {
					select {
					case <-ctx.Done():
						return
					case err := <-errs:
						if err != nil {
							fmt.Fprintf(out, "botropolisd: %v\n", err)
							_ = shutdowner.Shutdown(fx.ExitCode(1))
							return
						}
					}
				}
			}()
			fmt.Fprintf(out, "botropolisd: serving %s for %s\n", cfg.Socket, cfg.Home)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			_ = listener.Close()
			<-done
			_ = os.Remove(cfg.Socket)
			if err := d.Flush(); err != nil {
				fmt.Fprintf(out, "botropolisd: saving the ledger: %v\n", err)
			}
			return nil
		},
	})
}

// load reads the home, watches it, and reads it again once the watch is
// up: what changed between the first scan and the watch raised no event
// (item 124).
func load(ctx context.Context, d *daemon.Daemon, errs chan<- error) error {
	if err := d.Rescan(); err != nil {
		return err
	}
	go func() { errs <- d.Watch(ctx) }()
	if err := d.WaitWatching(ctx); err != nil {
		return nil
	}
	return d.Rescan()
}

// closeUpPath keeps the close-up beside the ledger, and nowhere when
// there is no ledger: the same default-home rule (item 86).
func closeUpPath(ledger string) string {
	if ledger == "" {
		return ""
	}
	return state.CloseUpPath()
}

// statsSource opens the statistics store when statistics are on and
// nothing about them is owed, and is nil otherwise: while the on prompt
// is owed the store stays closed, so its last write still marks where a
// gap starts. It is opened here, once, rather than when first written,
// so a private run's directory is never made again after it is removed.
// Item 110. A home the run was pointed at keeps none, like the ledger
// and the groups (item 165).
// The store ingests only what changed when the loader says what did
// (item 205).
var _ state.PathIngester = (*stats.Store)(nil)

// statsSource also says what became of the store. One that will not
// open leaves statistics off, as before, but no longer silently: the
// refusal goes to log, naming the file, and comes back as the store's
// state (item 190).
func statsSource(cfg *config.Config, log io.Writer) (*stats.Store, state.StatsStore) {
	if (cfg.HomeSet && !statedir.IsPrivate()) || !cfg.Stats {
		return nil, state.StatsStore{State: state.StoreOff}
	}
	path := stats.Path()
	if stats.Owed(statedir.Dir(), true) != stats.NoPrompt {
		return nil, state.StatsStore{State: state.StoreOwed, Path: path}
	}
	openStores.Lock()
	defer openStores.Unlock()
	if s, ok := openStores.byPath[path]; ok {
		return s, state.StatsStore{State: state.StoreOpen, Path: path}
	}
	s, err := stats.Open(path)
	if err != nil {
		fmt.Fprintf(log, "botropolisd: statistics are off: %v\n", err)
		return nil, state.StatsStore{State: state.StoreRefused, Path: path, Reason: refusal(err)}
	}
	openStores.byPath[path] = s
	return s, state.StatsStore{State: state.StoreOpen, Path: path}
}

// refusal is why a store was refused, in words that say what to do.
func refusal(err error) string {
	var r *stats.Refusal
	if errors.As(err, &r) {
		return r.Detail
	}
	return "it could not be opened: " + err.Error()
}

// openStores is the statistics store each path was opened as, so every
// loader in a process shares one.
var openStores = struct {
	sync.Mutex
	byPath map[string]*stats.Store
}{byPath: map[string]*stats.Store{}}
