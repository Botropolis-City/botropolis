package render

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/hotkey"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/script"
	"botropolis.city/botropolis/pkg/sighold"
	"botropolis.city/botropolis/pkg/single"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

const (
	DefaultWidth  = 1100
	DefaultHeight = 760
	x11Class      = "Botropolis"
	x11Instance   = "botropolis"
)

type Options struct {
	Layout     *city.Layout
	LayoutPath string
	Actor      Actor
	// Feed delivers snapshots and, before each, the events logged since
	// the last delivery, and says when its daemon is out of sync, which
	// stops the map until a snapshot from one in sync (item 152).
	Feed func(ctx context.Context, offer func(state.Snapshot), events func([]events.Event), outOfSync func(*proto.Mismatch))
	// Glance tells the feed whether the city can be seen, so a hidden
	// city lets its subscription go but for a glance every so often
	// (item 197).
	Glance func(hidden bool)
	// Loading asks the daemon how far its first load has got, for the
	// panel the window shows until the city opens (item 159).
	Loading func() (state.LoadProgress, error)
	// RestartDaemon restarts the daemon the dialog found out of sync,
	// and returns once one of this build's answers (item 152).
	RestartDaemon func(context.Context) error
	// Home is the directory holding .claude that the feed reads; the
	// usage boats and Summary read from it too.
	Home       string
	Width      int
	Height     int
	Projection city.Projection
	// Screenshot, when set, renders one frame to this PNG and exits;
	// Keys are pressed first, one per frame.
	Screenshot string
	Keys       []string
	// Hover parks the pointer at a window position for a scripted frame,
	// because --keys cannot move a mouse and some of the map only reacts
	// to one. HoverSet says whether it was asked for, since 0,0 is a
	// perfectly good place to point.
	Hover    city.Point
	HoverSet bool
	// Record, when set, writes a frame every tenth of a second into the
	// directory for RecordSeconds, pressing Keys two seconds apart, then
	// exits.
	Record        string
	RecordSeconds float64
	// Script is a clip's choreography, played while recording.
	Script []script.Cue
	// RecordFPS is how many of the 30 ticks a second are written; it
	// divides 30. 0 is the original ten.
	RecordFPS int
	// Scale is the chrome and pixel scale; 0 follows the display.
	Scale float64
	// Liveliness is how much the city moves; still stops every
	// animation and keeps the colours. Item 89.
	Liveliness city.Liveliness
	// Detail is how much of the city is drawn; plain drops the scenery.
	Detail city.Detail
	// HideGroupLines hides the line each group draws across its
	// districts, which are on unless the settings say otherwise.
	HideGroupLines bool
	// HideLabels starts the city with every label off (item 102).
	HideLabels bool
	// GroupSource builds what New session guesses a group from, for the
	// sessions on the map: the history detected once, and the recently
	// used directories.
	GroupSource func(sessions []state.Session, now time.Time) (*group.Guesser, []string)
	// JournalHours is how long away the map must have been for the
	// journal to open by itself; LastCloseUp reads the last close-up and
	// LastReply a session's last reply's last paragraph (item 86).
	JournalHours int
	LastCloseUp  func() (state.CloseUp, error)
	LastReply    func(id string) string
	// Ladder is how long a stopped session rests before it is shuttered,
	// and before it is overgrown; zero keeps the defaults. Item 86.
	Ladder state.Thresholds
	// UsageRefresh is how often the city probes for usage on its own;
	// zero leaves it to the u key, and the boats away until called.
	// Item 79.
	UsageRefresh time.Duration
	// Signage is how a session's title reaches the map. Phase 21 item
	// 38's prototypes, for comparison frames.
	Signage city.Signage
	// DailyBudget is the daily spend in USD the strip and plant measure
	// against; 0 for none.
	DailyBudget float64
	// Requests are what other commands ask of this, the one city, and
	// OpenFinder opens it with the finder showing. Item 82.
	Requests   <-chan single.Request
	OpenFinder bool
	// Desktop raises the window and holds the global shortcut, where the
	// desktop has a way to; nil where it has none, and DesktopErr says
	// why. GlobalShortcut is the combination it grabs as the city starts,
	// empty for none. Item 84.
	Desktop        hotkey.Desktop
	DesktopErr     error
	GlobalShortcut string
	// FindChords are the finder's chords; nil is this system's defaults.
	FindChords *config.Chords
	// Search finds sessions by what was said in them, for the finder.
	// Item 82.
	Search func(find.Query) ([]find.Hit, error)
	// Settings is the in-app settings panel's rows; Apply persists a change.
	Settings ui.Settings
	Apply    func(ui.Setting) error
	// StatsOn is whether statistics are on; StatsOwed is which prompt
	// the state directory owes, StatsSince when a kept database was last
	// written, and StatsChoose what answers a prompt. Item 110.
	StatsOn     bool
	StatsOwed   func(on bool) stats.Prompt
	StatsSince  func() (time.Time, bool)
	StatsChoose func(p stats.Prompt, keep bool) error
}

func Run(ctx context.Context, opts Options) error {
	scene := city.NewScene(opts.Layout)
	scene.SetProjection(opts.Projection)
	scene.SetBudget(opts.DailyBudget)
	width, height := opts.Width, opts.Height
	if width <= 0 || height <= 0 {
		width, height = DefaultWidth, DefaultHeight
	}
	scale := opts.Scale
	if scale <= 0 {
		scale = ebiten.Monitor().DeviceScaleFactor()
	}
	theme := ui.NewTheme(scale)
	scene.Resize(float64(width)*theme.Scale, float64(height)*theme.Scale)

	faces, err := newFaces(theme)
	if err != nil {
		return err
	}
	save := func(l *city.Layout) {
		if opts.LayoutPath != "" {
			_ = l.Save(opts.LayoutPath)
		}
	}
	sprites, loadErr := loadSprites()
	if loadErr != nil {
		return loadErr
	}
	kitSprites, err := loadKitsLazily()
	if err != nil {
		return err
	}
	game := NewGame(scene, opts.Actor, theme, faces, save, sprites)
	game.kits = kitSprites
	game.home = opts.Home
	game.search = opts.Search
	game.chords = opts.FindChords
	if opts.GroupSource != nil {
		game.groupSource = func() (*group.Guesser, []string) {
			return opts.GroupSource(game.scene.Snapshot().Sessions, game.scene.Clock())
		}
	}
	game.requests = opts.Requests
	if opts.Desktop != nil {
		game.shortcut = opts.Desktop
		game.win = raisedWindow{raise: opts.Desktop.Raise}
	}
	if opts.OpenFinder {
		game.openFinder()
	}
	if opts.Screenshot != "" || opts.Record != "" {
		// A scripted frame cannot wait for a search to come back, so it
		// searches on the frame, as soon as it is typed.
		game.dispatch = func(run func()) { run() }
		game.findAtOnce = true
		// And it opens on a read history, so its first frame stands on
		// the heights the rest of it will (item 202).
		scene.SetOpenOnRead(!framesOpenUnread)
	}
	game.screenshot = opts.Screenshot
	game.script = opts.Keys
	game.hover, game.hoverSet = opts.Hover, opts.HoverSet
	game.record = opts.Record
	if len(opts.Script) > 0 {
		game.player = newPlayer(opts.Script, city.Point{X: float64(width) * theme.Scale * 0.5, Y: float64(height) * theme.Scale * 0.5})
	}
	if opts.Record != "" {
		scene.SetClock(game.now, time.Local)
	}
	game.recordFrames = int(opts.RecordSeconds * 30)
	game.recordEvery = 3
	if opts.RecordFPS > 0 {
		game.recordEvery = 30 / opts.RecordFPS
	}
	game.setLiveliness(opts.Liveliness)
	scene.SetDetail(opts.Detail)
	scene.SetGroupLines(!opts.HideGroupLines)
	scene.SetLabels(!opts.HideLabels)
	game.journalHours, game.lastCloseUp, game.lastReply = opts.JournalHours, opts.LastCloseUp, opts.LastReply
	if opts.Ladder != (state.Thresholds{}) {
		scene.SetLadder(opts.Ladder)
	}
	game.signage = opts.Signage
	game.usageEvery = opts.UsageRefresh
	// A still or a film shows the boats on station whatever the setting:
	// the frame is there to show the city, and nothing can call them up
	// in one.
	scene.SetGaugesManual(opts.UsageRefresh <= 0 && opts.Screenshot == "" && opts.Record == "")
	// Read the cache at startup so the river is not empty on the first
	// frame. No probe: this is a file read. Item 49.
	game.readUsage()
	game.settings = opts.Settings
	game.apply = opts.Apply
	game.stats = statsHooks{on: opts.StatsOn, owed: opts.StatsOwed, since: opts.StatsSince, choose: opts.StatsChoose}
	game.setStatsOn(opts.StatsOn)
	game.desktopErr = opts.DesktopErr
	game.useShortcut(opts.GlobalShortcut)

	game.restartDaemon = opts.RestartDaemon
	game.glance = opts.Glance
	var feed func(context.Context, func(state.Snapshot), func([]events.Event))
	if opts.Feed != nil {
		feed = func(ctx context.Context, offer func(state.Snapshot), fresh func([]events.Event)) {
			opts.Feed(ctx, offer, fresh, game.OutOfSync)
		}
	}
	stopFeed := startFeed(ctx, feed, game.Offer, game.AddEvents)
	defer stopFeed()
	if opts.Loading != nil {
		go watchLoading(loadingEvery, func() bool { return game.opened.Load() || ctx.Err() != nil }, opts.Loading, game.loading.Store)
	}
	// A signal ends the loop, never the process: Run returns, and the
	// private state directory a caller made is removed (item 114).
	go func() {
		<-ctx.Done()
		game.Stop()
	}()

	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowSize(width, height)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(liveTPS)
	// The loop stays runnable and throttles itself: Ebitengine can only
	// gate on focus, and a window manager is free to leave a hidden
	// window focused, which is the case that ran away. See watching.go.
	ebiten.SetRunnableOnUnfocused(true)
	// A frame that repeats the one before it is skipped rather than
	// redrawn, which needs the last frame left on the glass.
	ebiten.SetScreenClearedEveryFrame(false)
	// Ebitengine runs its loop on this thread, the main one, and opens
	// the input devices from it: a signal arriving during an open ended
	// the city (item 128).
	release := sighold.Hold()
	err = ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		X11ClassName:    x11Class,
		X11InstanceName: x11Instance,
	})
	release()
	game.finishSharing()
	save(opts.Layout)
	if err != nil && err != ebiten.Termination {
		return err
	}
	if opts.Record != "" {
		fmt.Fprintln(os.Stderr, game.clipReport())
	}
	return nil
}

// startFeed runs feed until ctx ends or stop is called. stop returns
// only once feed has, because the feed writes the ledger into the state
// directory, and a caller removes a private one as soon as Run returns
// (item 114).
func startFeed(ctx context.Context, feed func(context.Context, func(state.Snapshot), func([]events.Event)), offer func(state.Snapshot), fresh func([]events.Event)) (stop func()) {
	feedCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	if feed == nil {
		close(done)
	} else {
		go func() {
			defer close(done)
			feed(feedCtx, offer, fresh)
		}()
	}
	return func() {
		cancel()
		<-done
	}
}
