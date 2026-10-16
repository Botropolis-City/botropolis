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
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/hotkey"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/script"
	"botropolis.city/botropolis/pkg/single"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/tui"
	"botropolis.city/botropolis/pkg/ui"
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
}

var Module = fx.Module("botropolis",
	fx.Provide(
		config.NewViper,
		newLoader,
		appd.NewProbes,
		fx.Annotate(newServices, fx.As(new(cli.Services)), fx.As(new(cli.CityServices)), fx.As(new(cli.BarServices)), fx.As(new(cli.NotifyServices)), fx.As(new(cli.TUIServices)), fx.As(new(cli.DoctorServices)), fx.As(new(cli.EventsServices)), fx.As(new(cli.FindServices))),
		fx.Annotate(cli.NewStatusCLI, fx.ResultTags(`name:"status"`)),
		fx.Annotate(cli.NewInstallHooksCLI, fx.ResultTags(`name:"installHooks"`)),
		fx.Annotate(cli.NewSessionCLIs, fx.ResultTags(`name:"sessions"`)),
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
			subs := append([]*cobra.Command{p.Status, p.Hooks, p.City, p.Bar, p.Notify, p.TUI, p.Doctor, p.Events, p.Find, p.Version, p.Shell}, p.Sessions...)
			return cli.NewRootCLI(p.Viper, subs...)
		},
	),
)

func Run(ctx context.Context, args []string) int {
	var root *cobra.Command
	app := fx.New(
		Module,
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		fx.Populate(&root),
	)
	if err := app.Err(); err != nil {
		_, _ = os.Stderr.WriteString("botropolis: " + err.Error() + "\n")
		return 1
	}
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = os.Stderr.WriteString("botropolis: " + err.Error() + "\n")
		return exitCode(err)
	}
	return 0
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

func (s *services) Sessions(cfg *config.Config) cli.SessionsRunner {
	return commands.NewSessions(cfg.Home, control.Default(cfg.Terminal))
}

func (s *services) source(cfg *config.Config) commands.DaemonOrDirect {
	return commands.DaemonOrDirect{
		Home: cfg.Home, Socket: cfg.Socket, Probes: s.probes,
		Direct: appd.Harnesses(cfg, s.probes).Load, Now: time.Now, Notice: os.Stderr,
	}
}

func (s *services) Bar(cfg *config.Config) cli.BarRunner {
	source := s.source(cfg)
	feed := commands.Feed{Socket: cfg.Socket, Source: source, Poll: 2 * time.Second, Retry: 5 * time.Second}
	return commands.NewBar(source, feed.Run)
}

func (s *services) Notify(cfg *config.Config) cli.NotifyRunner {
	feed := commands.Feed{Socket: cfg.Socket, Source: s.source(cfg), Poll: 2 * time.Second, Retry: 5 * time.Second}
	n := commands.NewNotify(feed.Run, commands.NotifierFor(runtime.GOOS))
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
			client, err := proto.Dial(sock)
			if err != nil {
				return err
			}
			return client.Close()
		},
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
	sessions := commands.NewSessions(r.config.Home, control.Default(r.config.Terminal))
	return tui.Run(ctx, feed.Run, sessions)
}

func (s *services) City(cfg *config.Config) cli.CityRunner {
	return &cityRunner{services: s, config: cfg}
}

type cityRunner struct {
	services *services
	config   *config.Config
}

// xvfbScreen is the virtual display for a window of the given size, or
// of the default size when none was asked for.
func xvfbScreen(width, height int) string {
	if width <= 0 || height <= 0 {
		width, height = render.DefaultWidth, render.DefaultHeight
	}
	return fmt.Sprintf("-screen 0 %dx%dx24", width, height)
}

// headlessEnv marks the re-executed child so it does not re-exec again.
const headlessEnv = "BOTROPOLIS_HEADLESS_CHILD"

// runHeadless re-runs this same command under xvfb-run, on a virtual X
// display of the window's size, with the desktop's displays hidden from
// it, so no window opens.
func runHeadless(cmd *cobra.Command) error {
	xvfb, err := exec.LookPath("xvfb-run")
	if err != nil {
		return fmt.Errorf("--headless needs xvfb-run (package xorg-server-xvfb on Arch, xvfb on Debian): %w", err)
	}
	width, height, err := cli.Window(cmd)
	if err != nil {
		return err
	}
	args := append([]string{"-a", "-s", xvfbScreen(width, height), os.Args[0]}, os.Args[1:]...)
	child := exec.CommandContext(cmd.Context(), xvfb, args...)
	child.Stdout, child.Stderr, child.Stdin = cmd.OutOrStdout(), cmd.ErrOrStderr(), os.Stdin
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "DISPLAY=") && !strings.HasPrefix(kv, "WAYLAND_DISPLAY=") {
			env = append(env, kv)
		}
	}
	child.Env = append(env, headlessEnv+"=1")
	return child.Run()
}

func (c *cityRunner) Run(cmd *cobra.Command) error {
	if cli.Headless(cmd) && os.Getenv(headlessEnv) == "" {
		return runHeadless(cmd)
	}
	layoutPath := city.LayoutPath()
	layout, err := city.LoadLayout(layoutPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "botropolis: %v; starting from an empty layout\n", err)
		layout = city.NewLayout()
	}
	actor := &liveActor{home: c.config.Home, groups: appd.GroupsPath(c.config)}
	actor.terminal.Store(c.config.Terminal)
	feed := commands.Feed{
		Socket:  c.config.Socket,
		Source:  c.services.source(c.config),
		Poll:    2 * time.Second,
		Retry:   5 * time.Second,
		OnError: func(err error) { fmt.Fprintf(cmd.ErrOrStderr(), "botropolis: %v\n", err) },
		Since:   func() time.Time { return layout.Seen },
	}
	projection, ok := city.ParseProjection(c.config.Projection)
	if !ok {
		return fmt.Errorf("unknown projection %q: use iso or top", c.config.Projection)
	}
	detail, ok := city.ParseDetail(c.config.Detail)
	if !ok {
		return fmt.Errorf("unknown detail %q: use full or plain", c.config.Detail)
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
		Feed: func(ctx context.Context, offer func(state.Snapshot), fresh func([]events.Event)) {
			feed.Events = fresh
			feed.Run(ctx, offer)
		},
		Home:           c.config.Home,
		FindChords:     &chords,
		Search:         commands.NewFind(c.config.Socket, c.config.Home, c.services.source(c.config), time.Now).Search,
		Projection:     projection,
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
		ReducedMotion:  c.config.ReducedMotion,
		DailyBudget:    c.config.DailyBudget,
		Detail:         detail,
		HideGroupLines: !c.config.GroupLines,
		GroupSource:    groupSource(c.config),
		Ladder:         state.Thresholds{Shuttered: time.Duration(c.config.ShutteredDays) * 24 * time.Hour, Overgrown: time.Duration(c.config.OvergrownDays) * 24 * time.Hour},
		UsageRefresh:   usageEvery,
		Signage:        signage,
		Settings:       settingsFor(c.config),
		Apply: func(s ui.Setting) error {
			key, value := settingValue(s)
			if key == config.KeyTerminal {
				actor.terminal.Store(value.(string))
			}
			return config.Save(c.config.File, key, value)
		},
	})
}

// liveActor builds its control on every action so a terminal chosen in
// the settings panel is used by the next attach.
type liveActor struct {
	home     string
	groups   string
	terminal atomic.Value
}

func (a *liveActor) Do(action city.Action) error {
	terminal, _ := a.terminal.Load().(string)
	sessions := commands.NewSessions(a.home, control.Default(terminal))
	return commands.Actor{Sessions: sessions, GroupStore: a.groups}.Do(action)
}

const (
	autoValue = "auto"
	offValue  = "off"
)

// settingsFor is the settings panel's rows from the loaded config.
func settingsFor(cfg *config.Config) ui.Settings {
	onOff := "off"
	if cfg.ReducedMotion {
		onOff = "on"
	}
	scale := autoValue
	if cfg.RenderScale > 0 {
		scale = strconv.FormatFloat(cfg.RenderScale, 'g', -1, 64)
	}
	lines := "on"
	if !cfg.GroupLines {
		lines = "off"
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
		{Key: config.KeyReducedMotion, Label: "reduced motion", Options: []string{"off", "on"}, Value: onOff},
		{Key: config.KeyRenderScale, Label: "render scale", Options: withValue([]string{autoValue, "1", "1.25", "1.5", "2"}, scale), Value: scale},
		{Key: config.KeyProjection, Label: "projection", Options: []string{"iso", "top"}, Value: cfg.Projection},
		{Key: config.KeyDetail, Label: "detail", Options: []string{"full", "plain"}, Value: cfg.Detail},
		{Key: config.KeyShutteredDays, Label: "shuttered days", Options: withValue([]string{"1", "3", "7", "14", "30"}, strconv.Itoa(cfg.ShutteredDays)), Value: strconv.Itoa(cfg.ShutteredDays)},
		{Key: config.KeyOvergrownDays, Label: "overgrown days", Options: withValue([]string{"7", "14", "30", "60", "90", "180"}, strconv.Itoa(cfg.OvergrownDays)), Value: strconv.Itoa(cfg.OvergrownDays)},
		{Key: config.KeyTerminal, Label: "terminal", Options: withValue(append([]string{autoValue}, control.Native().KnownTerminals()...), terminal), Value: terminal},
		{Key: config.KeyGroupLines, Label: "group lines", Options: []string{"on", "off"}, Value: lines},
		{Key: config.KeyUsageRefresh, Label: "usage refresh", Options: withValue([]string{config.UsageManual, "5m", "15m", "30m", "1h"}, cfg.UsageRefresh), Value: cfg.UsageRefresh},
		{Key: config.KeyGlobalShortcut, Label: "global shortcut", Value: shortcut},
	})
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

// settingValue is the config value a panel row stands for.
func settingValue(s ui.Setting) (string, any) {
	switch s.Key {
	case config.KeyReducedMotion, config.KeyGroupLines:
		return s.Key, s.Value == "on"
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
