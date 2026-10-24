package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/statedir"
	"botropolis.city/botropolis/pkg/stats"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"botropolis.city/botropolis/pkg/appd"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/cli"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/control"
	"botropolis.city/botropolis/pkg/daemonctl"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/hotkey"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/script"
	"botropolis.city/botropolis/pkg/single"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/tui"
	"botropolis.city/botropolis/pkg/ui"
	"botropolis.city/botropolis/pkg/xvfb"

	"github.com/mattn/go-isatty"
)

type cliParams struct {
	fx.In
	Viper    *viper.Viper
	Status   *cobra.Command   `name:"status"`
	Hooks    *cobra.Command   `name:"installHooks"`
	Sessions []*cobra.Command `name:"sessions"`
	City     *cobra.Command   `name:"city"`
	Bar      *cobra.Command   `name:"bar"`
	Notify   *cobra.Command   `name:"notify"`
	TUI      *cobra.Command   `name:"tui"`
	Doctor   *cobra.Command   `name:"doctor"`
	Events   *cobra.Command   `name:"events"`
	Find     *cobra.Command   `name:"find"`
	Version  *cobra.Command   `name:"version"`
	Shell    *cobra.Command   `name:"shellInit"`
	Setup    *cobra.Command   `name:"setup"`
}

// inTerminal is whether botropolis runs in a terminal, where attach
// attaches rather than opening one. Item 167.
func inTerminal(*cobra.Command) bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

func newSessionCLIs(load cli.Loader, services cli.Services) []*cobra.Command {
	return cli.NewSessionCLIs(load, services, inTerminal)
}

func newSetupCLI(load cli.Loader, services cli.SetupServices) *cobra.Command {
	return cli.NewSetupCLI(load, services, os.Getenv)
}

var Module = fx.Module("botropolis",
	fx.Provide(
		config.NewViper,
		newLoader,
		appd.NewProbes,
		fx.Annotate(newServices, fx.As(new(cli.Services)), fx.As(new(cli.CityServices)), fx.As(new(cli.BarServices)), fx.As(new(cli.NotifyServices)), fx.As(new(cli.TUIServices)), fx.As(new(cli.DoctorServices)), fx.As(new(cli.EventsServices)), fx.As(new(cli.FindServices)), fx.As(new(cli.SetupServices))),
		fx.Annotate(cli.NewStatusCLI, fx.ResultTags(`name:"status"`)),
		fx.Annotate(cli.NewInstallHooksCLI, fx.ResultTags(`name:"installHooks"`)),
		fx.Annotate(newSessionCLIs, fx.ResultTags(`name:"sessions"`)),
		fx.Annotate(newSetupCLI, fx.ResultTags(`name:"setup"`)),
		fx.Annotate(cli.NewCityCLI, fx.ResultTags(`name:"city"`)),
		fx.Annotate(cli.NewBarCLI, fx.ResultTags(`name:"bar"`)),
		fx.Annotate(cli.NewNotifyCLI, fx.ResultTags(`name:"notify"`)),
		fx.Annotate(cli.NewTUICLI, fx.ResultTags(`name:"tui"`)),
		fx.Annotate(cli.NewDoctorCLI, fx.ResultTags(`name:"doctor"`)),
		fx.Annotate(cli.NewEventsCLI, fx.ResultTags(`name:"events"`)),
		fx.Annotate(cli.NewFindCLI, fx.ResultTags(`name:"find"`)),
		fx.Annotate(cli.NewVersionCLI, fx.ResultTags(`name:"version"`)),
		fx.Annotate(cli.NewShellInitCLI, fx.ResultTags(`name:"shellInit"`)),
		func(p cliParams) *cobra.Command {
			subs := append([]*cobra.Command{p.Status, p.Hooks, p.City, p.Bar, p.Notify, p.TUI, p.Doctor, p.Events, p.Find, p.Version, p.Shell, p.Setup}, p.Sessions...)
			return cli.NewRootCLI(p.Viper, subs...)
		},
	),
)

// NewRoot is botropolis's command tree, as Run executes it and the
// manual pages describe it.
func NewRoot() (*cobra.Command, error) {
	var root *cobra.Command
	app := fx.New(
		Module,
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		fx.Populate(&root),
	)
	return root, app.Err()
}

func Run(ctx context.Context, args []string) int {
	root, err := NewRoot()
	if err != nil {
		_, _ = os.Stderr.WriteString("botropolis: " + err.Error() + "\n")
		return 1
	}
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		if errors.Is(err, render.ErrRestart) {
			return restart()
		}
		_, _ = os.Stderr.WriteString(errorLine(err, nativeHint) + "\n")
		return exitCode(err)
	}
	return 0
}

// errorLine is what a failed command prints: for a daemon out of sync,
// both versions and what to do about it, on one line (item 152).
func errorLine(err error, hint func(*proto.Mismatch) string) string {
	var mismatch *proto.Mismatch
	if errors.As(err, &mismatch) {
		return "botropolis: " + mismatch.Error() + ": " + hint(mismatch)
	}
	return "botropolis: " + err.Error()
}

func nativeHint(m *proto.Mismatch) string {
	return daemonctl.Hint(context.Background(), daemonctl.Native(), m)
}

// restart runs the city again, with its own arguments and environment,
// from its path on disk (item 152). It returns only when it cannot.
func restart() int {
	path, err := restartPath(os.Args[0], exec.LookPath, os.Executable)
	if err == nil {
		err = syscall.Exec(path, os.Args, os.Environ())
	}
	_, _ = os.Stderr.WriteString("botropolis: restarting: " + err.Error() + "\n")
	return 1
}

// restartPath is where the map runs again from: the path it was started
// by, looked up on the PATH when it was started by name, which after an
// upgrade is the new build. This executable's own path is the last
// resort, since on macOS it names the old version's directory.
func restartPath(args0 string, look func(string) (string, error), executable func() (string, error)) (string, error) {
	if strings.ContainsRune(args0, os.PathSeparator) {
		return args0, nil
	}
	if path, err := look(args0); err == nil {
		return path, nil
	}
	return executable()
}

const ExitUntrusted = 3

func exitCode(err error) int {
	if errors.Is(err, control.ErrUntrusted) {
		return ExitUntrusted
	}
	return 1
}

func newLoader(v *viper.Viper) cli.Loader {
	return func() (*config.Config, error) { return config.New(v) }
}

type services struct {
	probes state.Probes
}

func newServices(probes state.Probes) *services {
	return &services{probes: probes}
}

func (s *services) Status(cfg *config.Config) cli.StatusRunner {
	return commands.NewStatus(s.source(cfg))
}

func (s *services) Events(cfg *config.Config) cli.EventsRunner {
	return commands.NewEvents(cfg.Socket)
}

func (s *services) Find(cfg *config.Config) cli.FindRunner {
	return commands.NewFind(cfg.Socket, cfg.Home, s.source(cfg), time.Now).WithCity(single.SocketPath(), startCityWithFinder)
}

// openFinderEnv asks a city started by botropolis find to open with the
// finder showing.
const openFinderEnv = "BOTROPOLIS_OPEN_FINDER"

// startCityWithFinder becomes the city, with the finder open: botropolis
// find with no words, when no city is running. It replaces this process
// rather than starting another, keeping the arguments given, so --home
// and the rest still apply.
func startCityWithFinder() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := append([]string(nil), os.Args...)
	for i, a := range args {
		if i > 0 && a == "find" {
			args[i] = "city"
			break
		}
	}
	return syscall.Exec(exe, args, append(os.Environ(), openFinderEnv+"=1"))
}

func (s *services) Hooks(cfg *config.Config) cli.HooksRunner {
	return commands.NewHooks(cfg.Home, cfg.HookCommand)
}

func (s *services) Daemon(cfg *config.Config) cli.DaemonRunner {
	return commands.Daemon{
		GOOS:    runtime.GOOS,
		OnPath:  func(name string) bool { _, err := exec.LookPath(name); return err == nil },
		Running: func() bool { _, err := proto.Ask(cfg.Socket); return err == nil },
		Run: func(argv []string) error {
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
			return cmd.Run()
		},
	}
}

func (s *services) Sessions(cfg *config.Config) cli.SessionsRunner {
	return commands.NewSessions(cfg.Home, control.Default(cfg.Terminal)).WithForget(pruneForgetter(cfg)).WithDemolished(demolisher(cfg))
}

func (s *services) source(cfg *config.Config) commands.DaemonOrDirect {
	return s.sourceAt(cfg, appd.Paced)
}

func (s *services) sourceAt(cfg *config.Config, speed appd.Speed) commands.DaemonOrDirect {
	return commands.DaemonOrDirect{
		Home: cfg.Home, Socket: cfg.Socket, Probes: s.probes,
		Direct: appd.Harnesses(cfg, s.probes, speed).Load, Now: time.Now, Notice: os.Stderr,
	}
}

// readSpeed is how fast a city reads its history: a capture at full
// speed, since it waits on the read and leaves nothing else waiting
// (item 240), and an open city at half a core.
func readSpeed(screenshot, record string) appd.Speed {
	if screenshot != "" || record != "" {
		return appd.FullSpeed
	}
	return appd.Paced
}

func (s *services) Bar(cfg *config.Config) cli.BarRunner {
	source := s.source(cfg)
	feed := commands.Feed{Socket: cfg.Socket, Source: source, Poll: 2 * time.Second, Retry: 5 * time.Second}
	return commands.NewBar(source, feed.RunRefusing).WithTriage(barLayout, time.Now)
}

// barLayout is the city's triage marks; a layout that cannot be read
// sets nothing aside.
func barLayout() *city.Layout {
	layout, err := city.LoadLayout(city.LayoutPath())
	if err != nil {
		return nil
	}
	return layout
}

func (s *services) Notify(cfg *config.Config) cli.NotifyRunner {
	feed := commands.Feed{Socket: cfg.Socket, Source: s.source(cfg), Poll: 2 * time.Second, Retry: 5 * time.Second}
	n := commands.NewNotify(feed.RunRefusing, commands.NotifierFor(runtime.GOOS))
	n.OnError = func(err error) { fmt.Fprintf(os.Stderr, "botropolis notify: %v\n", err) }
	return n
}

func (s *services) Doctor(cfg *config.Config) cli.DoctorRunner {
	return commands.Doctor{
		Home: cfg.Home, Socket: cfg.Socket, Terminal: cfg.Terminal, HookCommand: cfg.HookCommand, CodexHome: cfg.CodexHome,
		Desktop: control.Native(),
		Getenv:  os.Getenv,
		OnPath:  func(name string) bool { _, err := exec.LookPath(name); return err == nil },
		Dial: func(sock string) error {
			peer, err := proto.Ask(sock)
			if err != nil {
				return err
			}
			if err := proto.Compare(peer, proto.Self()); err != nil {
				mismatch := err.(*proto.Mismatch)
				mismatch.Socket = sock
				return mismatch
			}
			return nil
		},
		RestartHint: nativeHint,
	}
}

func (s *services) TUI(cfg *config.Config) cli.TUIRunner {
	return &tuiRunner{services: s, config: cfg}
}

type tuiRunner struct {
	services *services
	config   *config.Config
}

func (r *tuiRunner) Run(ctx context.Context) error {
	feed := commands.Feed{Socket: r.config.Socket, Source: r.services.source(r.config), Poll: 2 * time.Second, Retry: 5 * time.Second}
	sessions := commands.NewSessions(r.config.Home, control.Default(r.config.Terminal)).WithDemolished(demolisher(r.config))
	return tui.Run(ctx, feed.RunRefusing, sessions)
}

func (s *services) City(cfg *config.Config) cli.CityRunner {
	return &cityRunner{services: s, config: cfg}
}

type cityRunner struct {
	services *services
	config   *config.Config
}

// screenSize is the virtual display for a window of the given size, or
// of the default size when none was asked for.
func screenSize(width, height int) [2]int {
	if width <= 0 || height <= 0 {
		width, height = render.DefaultWidth, render.DefaultHeight
	}
	return [2]int{width, height}
}

// headlessEnv marks the re-executed child so it does not re-exec again.
const headlessEnv = "BOTROPOLIS_HEADLESS_CHILD"

// privateStateEnv hands the headless parent's private state directory
// to its child. It is plumbing between the two, not a setting, and the
// child honours it only alongside headlessEnv (item 114).
const privateStateEnv = "BOTROPOLIS_INTERNAL_PRIVATE_STATE"

// runHeadless re-runs this same command on a virtual X display of the
// window's size, with the desktop's displays hidden from it, so no
// window opens. The display is the one Xvfb reports taking, so cities
// started together never share one (item 126). The child works in
// stateDir, which this process made and removes, and the display's
// cookie is kept there too.
//
// Xvfb and the city run in a process group of their own, so a
// terminal's Ctrl-C reaches only this process and not Xvfb, which would
// take the city's display away mid-cleanup. On a signal this process
// kills that group, and only that group, waits until none of it can
// still write, and returns, so stateDir is removed after the city is
// gone (item 114).
func runHeadless(cmd *cobra.Command, stateDir string) error {
	width, height, err := cli.Window(cmd)
	if err != nil {
		return err
	}
	size := screenSize(width, height)
	server, err := xvfb.Start(stateDir, size[0], size[1])
	if err != nil {
		return fmt.Errorf("--headless: %w", err)
	}
	defer func() { _ = server.Close() }()
	child := exec.Command(os.Args[0], os.Args[1:]...)
	child.Stdout, child.Stderr, child.Stdin = cmd.OutOrStdout(), cmd.ErrOrStderr(), os.Stdin
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: server.Pid()}
	var env []string
	for _, kv := range os.Environ() {
		switch key, _, _ := strings.Cut(kv, "="); key {
		case "DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", privateStateEnv:
		default:
			env = append(env, kv)
		}
	}
	child.Env = append(append(env, server.Env()...), headlessEnv+"=1", privateStateEnv+"="+stateDir)
	if err := child.Start(); err != nil {
		return err
	}
	var waited error
	done := make(chan struct{})
	go func() { waited = child.Wait(); close(done) }()
	select {
	case <-done:
		return waited
	case <-cmd.Context().Done():
		group := server.Pid()
		_ = syscall.Kill(-group, syscall.SIGKILL)
		<-done
		_ = server.Close()
		waitGroupGone(group, 5*time.Second)
		return nil
	}
}

// waitGroupGone waits until nothing in group is left but zombies, which
// can write nothing, or until limit passes.
func waitGroupGone(group int, limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) && groupAlive(group) {
		time.Sleep(10 * time.Millisecond)
	}
}

func groupAlive(group int) bool {
	if err := syscall.Kill(-group, 0); err != nil {
		return false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return true
	}
	for _, e := range entries {
		stat, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		shut := strings.LastIndexByte(string(stat), ')')
		if shut < 0 {
			continue
		}
		fields := strings.Fields(string(stat[shut+1:]))
		if len(fields) >= 3 && fields[2] == strconv.Itoa(group) && fields[0] != "Z" {
			return true
		}
	}
	return false
}

func (c *cityRunner) Run(cmd *cobra.Command) error {
	if cli.Headless(cmd) && os.Getenv(headlessEnv) == "" {
		cleanup, err := statedir.MakePrivate(seedState(c.config))
		if err != nil {
			return err
		}
		defer func() { _ = cleanup() }()
		return runHeadless(cmd, statedir.Dir())
	}
	if dir := os.Getenv(privateStateEnv); dir != "" && os.Getenv(headlessEnv) != "" {
		defer statedir.Adopt(dir)()
	} else if notOwnCity(cmd, c.config) {
		cleanup, err := statedir.MakePrivate(seedState(c.config))
		if err != nil {
			return err
		}
		defer func() { _ = cleanup() }()
	}
	layoutPath := city.LayoutPath()
	layout, err := city.LoadLayout(layoutPath)
	if errors.Is(err, statedir.ErrNewer) {
		return err
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "botropolis: %v; starting from an empty layout\n", err)
		layout = city.NewLayout()
	}
	layout.SetTyping(typingFor(c.config.DistrictType, cmd.ErrOrStderr()))
	actor := &liveActor{home: c.config.Home, groups: appd.GroupsPath(c.config), socket: c.config.Socket, demolished: demolisher(c.config)}
	actor.terminal.Store(c.config.Terminal)
	recording, _ := cli.Record(cmd)
	feed := commands.Feed{
		Socket:  c.config.Socket,
		Source:  c.services.sourceAt(c.config, readSpeed(cli.Screenshot(cmd), recording)),
		Poll:    2 * time.Second,
		Retry:   5 * time.Second,
		OnError: func(err error) { fmt.Fprintf(cmd.ErrOrStderr(), "botropolis: %v\n", err) },
		Since:   func() time.Time { return layout.Seen },
	}
	detail, ok := city.ParseDetail(c.config.Detail)
	if !ok {
		return fmt.Errorf("unknown detail %q: use full or plain", c.config.Detail)
	}
	liveliness, ok := city.ParseLiveliness(c.config.Liveliness)
	if !ok {
		return fmt.Errorf("unknown liveliness %q: use still, calm, steady or lively", c.config.Liveliness)
	}
	usageEvery, err := config.ParseUsageRefresh(c.config.UsageRefresh)
	if err != nil {
		return err
	}
	recordDir, recordSeconds := cli.Record(cmd)
	fps, err := cli.FPS(cmd)
	if err != nil {
		return err
	}
	width, height, err := cli.Window(cmd)
	if err != nil {
		return err
	}
	var cues []script.Cue
	if path := cli.Script(cmd); path != "" {
		if cues, err = script.Load(path); err != nil {
			return err
		}
	}
	signage, ok := city.ParseSignage(cli.Signage(cmd))
	if !ok {
		return fmt.Errorf("unknown signage %q: use plates, gantry, board, plaque or hover", cli.Signage(cmd))
	}
	hoverX, hoverY, hoverSet := cli.Hover(cmd)
	hoverPoint := city.Point{X: hoverX, Y: hoverY}
	chords, err := config.FindChords(*c.config, runtime.GOOS, render.KnownKey)
	if err != nil {
		return err
	}
	// One city: a second one asks the first to come forward, or to open
	// the finder, and exits. A scripted city neither claims nor defers.
	// Item 82, slice 5.
	openFinder := os.Getenv(openFinderEnv) != ""
	first := single.Request{Op: single.Raise}
	if openFinder {
		first.Op = single.Find
	}
	scripted := cli.Screenshot(cmd) != "" || recordDir != "" || cli.Headless(cmd)
	one, deferred, err := single.Claim(single.SocketPath(), first, scripted)
	if err != nil {
		return err
	}
	if deferred {
		fmt.Fprintln(cmd.ErrOrStderr(), "the city is already open; brought it forward")
		return nil
	}
	var requests <-chan single.Request
	var desktop hotkey.Desktop
	var desktopErr error
	var shortcut string
	if one != nil {
		defer func() { _ = one.Close() }()
		requests = one.Requests()
		shortcut = c.config.GlobalShortcut
	}
	// The city reaches for the desktop of the display it is drawn on. The
	// one city grabs the combination the config holds; a scripted one
	// grabs only what its own keys record, so a frame can show the row
	// as it really behaves. Item 84.
	if one != nil || scripted {
		if desktop, desktopErr = hotkey.Open(runtime.GOOS, os.Getenv); desktop != nil {
			defer desktop.Close()
		}
	}
	return render.Run(cmd.Context(), render.Options{
		Requests:       requests,
		OpenFinder:     openFinder,
		Desktop:        desktop,
		DesktopErr:     desktopErr,
		GlobalShortcut: shortcut,
		Layout:         layout,
		LayoutPath:     layoutPath,
		Actor:          actor,
		Feed: func(ctx context.Context, offer func(state.Snapshot), fresh func([]events.Event), outOfSync func(*proto.Mismatch)) {
			feed.Events = fresh
			feed.Mismatch = outOfSync
			feed.Run(ctx, offer)
		},
		Loading: func() (state.LoadProgress, error) { return proto.Loading(c.config.Socket) },
		RestartDaemon: func(ctx context.Context) error {
			return daemonctl.Restart(ctx, daemonctl.Native(), c.config.Socket, proto.Self())
		},
		Home:           c.config.Home,
		FindChords:     &chords,
		Search:         commands.NewFind(c.config.Socket, c.config.Home, c.services.source(c.config), time.Now).Search,
		Projection:     city.Isometric,
		Screenshot:     cli.Screenshot(cmd),
		Keys:           cli.Keys(cmd),
		Hover:          hoverPoint,
		HoverSet:       hoverSet,
		Record:         recordDir,
		RecordSeconds:  recordSeconds,
		RecordFPS:      fps,
		Script:         cues,
		Width:          width,
		Height:         height,
		Scale:          c.config.RenderScale,
		Liveliness:     liveliness,
		DailyBudget:    c.config.DailyBudget,
		Detail:         detail,
		HideGroupLines: !c.config.GroupLines,
		HideLabels:     !c.config.Labels,
		GroupSource:    groupSource(c.config),
		JournalHours:   c.config.JournalHours,
		LastCloseUp:    func() (state.CloseUp, error) { return state.LoadCloseUp(state.CloseUpPath()) },
		Ladder:         state.Thresholds{Shuttered: time.Duration(c.config.ShutteredDays) * 24 * time.Hour, Overgrown: time.Duration(c.config.OvergrownDays) * 24 * time.Hour},
		UsageRefresh:   usageEvery,
		Signage:        signage,
		Settings:       settingsFor(c.config),
		StatsOn:        c.config.Stats,
		StatsOwed:      func(on bool) stats.Prompt { return stats.Owed(statedir.Dir(), on) },
		StatsSince:     func() (time.Time, bool) { return stats.OffSince(statedir.Dir()) },
		StatsChoose:    statsChoice(c.config.File, statedir.Dir(), time.Now),
		Apply: func(s ui.Setting) error {
			if s.Key == config.KeyTerminal {
				_, value := settingValue(s)
				actor.terminal.Store(value.(string))
			}
			return saveSetting(c.config.File, s)
		},
	})
}

// notOwnCity is whether a run draws something other than the person's
// own city, a frame or a home it was pointed at, and so must keep its
// state to itself (item 111).
func notOwnCity(cmd *cobra.Command, cfg *config.Config) bool {
	record, _ := cli.Record(cmd)
	return cli.Headless(cmd) || record != "" || cli.Script(cmd) != "" || cfg.HomeSet
}

// seedState is where a private run's state is copied from: the state
// directory inside a home it was pointed at, so a fixture home can carry
// its own layout, ledger and groups, or nothing.
func seedState(cfg *config.Config) string {
	if !cfg.HomeSet {
		return ""
	}
	return filepath.Join(cfg.Home, ".local", "state", "botropolis")
}

// liveActor builds its control on every action so a terminal chosen in
// the settings panel is used by the next attach.
type liveActor struct {
	home     string
	groups   string
	socket   string
	terminal atomic.Value
	// demolished has the ledger forget a session the map demolished
	// (item 144).
	demolished func(string) error
}

// closeUp asks the daemon at the actor's socket to record a close-up
// (item 86).
func (a *liveActor) closeUp(open, pinned []string) error {
	client, err := proto.Dial(a.socket)
	if err != nil {
		return fmt.Errorf("closing up needs the daemon: %w", err)
	}
	defer func() { _ = client.Close() }()
	return client.CloseUp(open, pinned)
}

// pack asks the daemon at the actor's socket to pack a session up or
// unpack it (item 86).
func (a *liveActor) pack(id string, packed bool) error {
	client, err := proto.Dial(a.socket)
	if err != nil {
		return fmt.Errorf("packing up needs the daemon: %w", err)
	}
	defer func() { _ = client.Close() }()
	return client.Pack(id, packed)
}

func (a *liveActor) Do(action city.Action) error {
	terminal, _ := a.terminal.Load().(string)
	sessions := commands.NewSessions(a.home, control.Default(terminal)).WithDemolished(a.demolished)
	return commands.Actor{Sessions: sessions, GroupStore: a.groups, Pack: a.pack, CloseUp: a.closeUp}.Do(action)
}

const (
	autoValue = "auto"
	offValue  = "off"
)

// settingsFor is the settings panel's rows from the loaded config.
func settingsFor(cfg *config.Config) ui.Settings {
	scale := autoValue
	if cfg.RenderScale > 0 {
		scale = strconv.FormatFloat(cfg.RenderScale, 'g', -1, 64)
	}
	lines := "on"
	if !cfg.GroupLines {
		lines = "off"
	}
	labels := config.LabelsShown
	if !cfg.Labels {
		labels = config.LabelsHidden
	}
	terminal := cfg.Terminal
	if terminal == "" {
		terminal = autoValue
	}
	shortcut := cfg.GlobalShortcut
	if shortcut == "" {
		shortcut = offValue
	}
	return ui.NewSettings([]ui.Setting{
		{Key: config.KeyLiveliness, Label: "liveliness", Options: livelinessNames(), Value: cfg.Liveliness},
		{Key: config.KeyRenderScale, Label: "render scale", Options: withValue([]string{autoValue, "1", "1.25", "1.5", "2"}, scale), Value: scale},
		{Key: config.KeyDetail, Label: "detail", Options: []string{"full", "plain"}, Value: cfg.Detail},
		{Key: config.KeyShutteredDays, Label: "shuttered days", Options: withValue([]string{"1", "3", "7", "14", "30"}, strconv.Itoa(cfg.ShutteredDays)), Value: strconv.Itoa(cfg.ShutteredDays)},
		{Key: config.KeyOvergrownDays, Label: "overgrown days", Options: withValue([]string{"7", "14", "30", "60", "90", "180"}, strconv.Itoa(cfg.OvergrownDays)), Value: strconv.Itoa(cfg.OvergrownDays)},
		{Key: config.KeyTerminal, Label: "terminal", Options: withValue(append([]string{autoValue}, control.Native().KnownTerminals()...), terminal), Value: terminal},
		{Key: config.KeyGroupLines, Label: "group lines", Options: []string{"on", "off"}, Value: lines},
		{Key: config.KeyLabels, Label: "labels", Options: []string{config.LabelsShown, config.LabelsHidden}, Value: labels},
		{Key: config.KeyDistrictType, Label: "new districts", Options: withValue(append([]string{}, city.TypingModes...), cfg.DistrictType), Value: cfg.DistrictType},
		{Key: config.KeyStats, Label: "statistics", Options: []string{"on", "off"}, Value: statsValue(cfg.Stats)},
		{Key: config.KeyUsageRefresh, Label: "usage refresh", Options: withValue([]string{config.UsageManual, "15m", "30m", "1h"}, cfg.UsageRefresh), Value: cfg.UsageRefresh},
		{Key: config.KeyGlobalShortcut, Label: "global shortcut", Value: shortcut},
	})
}

// livelinessNames is the levels, from still to lively.
func livelinessNames() []string {
	var out []string
	for _, l := range city.LivelinessLevels {
		out = append(out, l.String())
	}
	return out
}

// withValue keeps a value that is not among the options at the end of
// the cycle, so the panel can show it without dropping it.
func withValue(options []string, value string) []string {
	for _, o := range options {
		if o == value {
			return options
		}
	}
	return append(options, value)
}

// saveSetting persists a panel row. Liveliness also clears reduced
// motion, which would otherwise hold the city still whatever the row
// said: the row is the newer choice. Item 89.
func saveSetting(file string, s ui.Setting) error {
	key, value := settingValue(s)
	if key == config.KeyLiveliness {
		if err := config.Save(file, config.KeyReducedMotion, false); err != nil {
			return err
		}
	}
	return config.Save(file, key, value)
}

// settingValue is the config value a panel row stands for.
func settingValue(s ui.Setting) (string, any) {
	switch s.Key {
	case config.KeyGroupLines, config.KeyStats:
		return s.Key, s.Value == "on"
	case config.KeyLabels:
		return s.Key, s.Value == config.LabelsShown
	case config.KeyRenderScale:
		if s.Value == autoValue {
			return s.Key, 0.0
		}
		f, _ := strconv.ParseFloat(s.Value, 64)
		return s.Key, f
	case config.KeyShutteredDays, config.KeyOvergrownDays:
		n, _ := strconv.Atoi(s.Value)
		return s.Key, n
	case config.KeyTerminal:
		if s.Value == autoValue {
			return s.Key, ""
		}
	case config.KeyGlobalShortcut:
		if s.Value == offValue {
			return s.Key, ""
		}
	}
	return s.Key, s.Value
}

// groupSource reads Claude Code's prompt history where it is kept, in
// memory, and the default home's groups, for New session to guess from
// (item 87). A home the run was pointed at has no groups of its own.
func groupSource(cfg *config.Config) func([]state.Session, time.Time) (*group.Guesser, []string) {
	return func(sessions []state.Session, now time.Time) (*group.Guesser, []string) {
		history, err := claude.ReadPromptHistory(filepath.Join(cfg.Home, ".claude", "history.jsonl"))
		if err != nil {
			return nil, nil
		}
		memory := group.Memory{}
		if path := appd.GroupsPath(cfg); path != "" {
			if loaded, err := group.Load(path); err == nil {
				memory = loaded
			}
		}
		return group.NewGuesser(state.GuessMembers(history, sessions, now), memory), state.RecentDirs(history)
	}
}

func statsValue(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// statsChoice answers a statistics prompt: the store keeps or deletes
// what it holds, then the setting is saved to the config file. Item 110.
func statsChoice(file, dir string, now func() time.Time) func(stats.Prompt, bool) error {
	return func(p stats.Prompt, keep bool) error {
		on := p == stats.OnPrompt
		var err error
		if on {
			err = stats.TurnOn(dir, keep, now())
		} else {
			err = stats.TurnOff(dir, keep)
		}
		if err != nil {
			return err
		}
		return config.Save(file, config.KeyStats, on)
	}
}

// demolisher has the daemon forget a demolished session from the ledger,
// or with no daemon to ask, notes it beside the ledger for the daemon's
// next start, since only the daemon writes the ledger. A home the run was
// pointed at keeps no ledger, so there is nothing to forget. Item 144.
func demolisher(cfg *config.Config) func(string) error {
	if cfg.HomeSet && !statedir.IsPrivate() {
		return func(string) error { return nil }
	}
	ledger, socket := state.LedgerPath(), cfg.Socket
	return func(session string) error {
		client, err := proto.Dial(socket)
		if err != nil {
			return state.NoteForgotten(ledger, session)
		}
		defer func() { _ = client.Close() }()
		return client.Forget(session)
	}
}

// pruneForgetter is how prune forgets a removed session's statistics:
// while they are off it notes the session beside data kept for later,
// without opening the database, and it never creates a database that is
// not there yet. Item 110.
func pruneForgetter(cfg *config.Config) func(string) error {
	if !cfg.Stats {
		return func(session string) error { return stats.NotePruned(statedir.Dir(), session) }
	}
	return func(session string) error {
		path := stats.Path()
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		s, err := stats.Open(path)
		if err != nil {
			return err
		}
		if err := s.Forget(session); err != nil {
			_ = s.Close()
			return err
		}
		return s.Close()
	}
}
