package render

import (
	"context"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/script"
	"botropolis.city/botropolis/pkg/state"
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
	// the last delivery.
	Feed func(ctx context.Context, offer func(state.Snapshot), events func([]events.Event))
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
	// ReducedMotion stops every animation and keeps the colours.
	ReducedMotion bool
	// Detail is how much of the city is drawn; plain drops the scenery.
	Detail city.Detail
	// Signage is how a session's title reaches the map. Phase 21 item
	// 38's prototypes, for comparison frames.
	Signage city.Signage
	// DailyBudget is the daily spend in USD the strip and plant measure
	// against; 0 for none.
	DailyBudget float64
	// Settings is the in-app settings panel's rows; Apply persists a change.
	Settings ui.Settings
	Apply    func(ui.Setting) error
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
	kitSprites, err := loadKits()
	if err != nil {
		return err
	}
	game := NewGame(scene, opts.Actor, theme, faces, save, sprites)
	game.kits = kitSprites
	game.home = opts.Home
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
	game.reduced = opts.ReducedMotion
	scene.SetDetail(opts.Detail)
	game.signage = opts.Signage
	// Read the cache at startup so the river is not empty on the first
	// frame. No probe: this is a file read. Item 49.
	game.readUsage()
	game.settings = opts.Settings
	game.apply = opts.Apply

	feedCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if opts.Feed != nil {
		go opts.Feed(feedCtx, game.Offer, game.AddEvents)
	}
	go func() {
		<-feedCtx.Done()
		save(opts.Layout)
		os.Exit(0)
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
	err = ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		X11ClassName:    x11Class,
		X11InstanceName: x11Instance,
	})
	save(opts.Layout)
	if err != nil && err != ebiten.Termination {
		return err
	}
	return nil
}
