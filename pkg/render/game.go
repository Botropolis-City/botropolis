package render

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/single"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

const (
	windowTitle = "Botropolis"
	pulsePeriod = 1.4
	titleChars  = 18
)

var (
	colorPlant      = color.NRGBA{0x4a, 0x3b, 0x2a, 0xff}
	colorPlantCore  = color.NRGBA{0xf0, 0xb4, 0x4c, 0xff}
	colorTower      = color.NRGBA{0x2f, 0x3a, 0x48, 0xff}
	colorTowerUsed  = color.NRGBA{0x5c, 0xc8, 0xb0, 0xff}
	colorLibrary    = color.NRGBA{0x3a, 0x33, 0x50, 0xff}
	colorLineFresh  = color.NRGBA{0xf0, 0xb4, 0x4c, 0xb0}
	colorLineCached = color.NRGBA{0x6c, 0xa8, 0xd8, 0x50}
	colorBeam       = color.NRGBA{0x5c, 0xc8, 0xb0, 0x70}
	colorWindow     = color.NRGBA{0xf2, 0xe6, 0xa8, 0xd0}
	colorWindowDark = color.NRGBA{0x1a, 0x1e, 0x28, 0xff}
	colorFlag       = color.NRGBA{0xe0, 0x50, 0x50, 0xff}
	colorPole       = color.NRGBA{0xc0, 0xc0, 0xc0, 0xff}
	colorSmoke      = color.NRGBA{0x9a, 0x9a, 0x9a, 0x70}
	colorRoad       = color.NRGBA{0x3c, 0x40, 0x4a, 0xff}
	colorGaugeBack  = color.NRGBA{0x14, 0x16, 0x1c, 0xe0}
	colorGaugeLow   = color.NRGBA{0x6c, 0xa8, 0xd8, 0xff}
	colorGaugeHigh  = color.NRGBA{0xe8, 0x6c, 0x4c, 0xff}
	colorBuilding   = color.NRGBA{0x2c, 0x33, 0x44, 0xff}
	colorLit        = color.NRGBA{0x3d, 0x5a, 0x80, 0xff}
	colorNeedsYou   = ui.DefaultPalette.NeedsYou
	colorUnattended = color.NRGBA{0x5b, 0x48, 0x8a, 0xff}
	colorBoarded    = color.NRGBA{0x30, 0x30, 0x34, 0xff}
	colorFill       = color.NRGBA{0x6c, 0xa8, 0xd8, 0xdd}
	colorCrane      = color.NRGBA{0xd8, 0xd0, 0x8c, 0xff}
	colorText       = ui.DefaultPalette.Text
	colorDim        = ui.DefaultPalette.Dim
	colorSelected   = color.NRGBA{0xff, 0xff, 0xff, 0xff}
)

type Actor interface {
	Do(action city.Action) error
}

type Game struct {
	// ledger is the statistics store as this window reads it, nil
	// while statistics are off (item 116).
	ledger *stats.Ledger
	// stopping is set by a signal: the loop ends at its next update, so
	// Run returns and whoever started it can clean up (item 114).
	stopping atomic.Bool
	// holdGL pins the GL libraries once the context exists, at the first
	// update, so Ebitengine's teardown cannot unmap them under Mesa's
	// threads; nil means glhold.Hold (item 125).
	holdGL func()
	glHeld sync.Once
	scene  *city.Scene
	// home is the directory holding .claude, the one the city is drawn
	// from; see Options.Home.
	home string
	// usageRead is the modification time of the usage cache last read.
	usageRead time.Time
	// usageEvery is how often the city probes for usage on its own; zero
	// leaves it to the u key. Item 79.
	usageEvery time.Duration
	// player plays a clip's script; play is this frame's share of it.
	player *player
	play   playFrame
	// clickedAt is when the scripted pointer last clicked, for the ripple
	// drawn under it.
	clickedAt float64
	// lit is the contextual highlight for the frame being drawn.
	lit city.Highlight
	// hover parks the pointer for a scripted frame; see Options.Hover.
	hover     city.Point
	hoverSet  bool
	actor     Actor
	theme     ui.Theme
	faces     *faces
	saveState func(*city.Layout)
	sprites   *sprites
	kits      *kits

	mu          sync.Mutex
	pending     *state.Snapshot
	notice      ui.Notice
	shownTitle  string
	stripLayout ui.Strip

	// screenshot mode: frames drawn since the first snapshot, and the result.
	screenshot string
	shotFrames int
	shotErr    error
	shotDone   bool
	// script is the keys --keys asked for, pressed one per frame before
	// the screenshot; scripted is this frame's.
	script   []string
	scripted ebiten.Key
	// record is the directory --record writes into, recordFrames how
	// many ticks to run and recorded how many have gone by.
	record       string
	recordFrames int
	recorded     int
	// recordEvery is how many ticks pass between written frames.
	recordEvery int
	// clipFrom and clipTo are the city's ticks at the first and last
	// draw the clip counted; see clipReport. Item 113.
	clipFrom, clipTo uint64
	// frames encodes recorded frames off the loop.
	frames *frameWriter
	// snap is a frame the p key asked for, saved on the next draw.
	snap string
	// viewKey is the overlay listing the six lenses; see ui.LayoutViewKey.
	// viewKeyRows is the layout as last drawn, so a click lands on the
	// rows the reader can actually see.
	viewKey     bool
	viewKeyRows ui.ViewKey
	// footerBar is the footer as last drawn, so a click lands on the
	// verb that is there. clicked is the key a clicked verb stands in
	// for this frame: the click runs the keyboard's path rather than a
	// second copy of it, which is what "one code path, not two" means.
	footerBar ui.Footer
	// waitingID is the session the footer's needs-you line names, as
	// last drawn. Item 91.
	waitingID string
	// waitingHovered is whether the line was drawn under the pointer.
	waitingHovered bool
	settingsPanel  ui.SettingsPanel
	searchPlate    city.Rect
	// usage is Claude Code's own cached view of the account's limits,
	// re-read off disk because reading a file is free. u makes Claude
	// Code refetch. Item 49.
	usage claude.Utilization
	// usageAskedAt is when the probe was last set going and usageInFlight
	// whether one has yet to come back. Bug 54: each probe is a real
	// Claude Code session while it runs, so asking repeatedly puts a
	// fleet on the river and re-plans the map under it. Both are read
	// and written under mu.
	usageAskedAt  time.Time
	usageInFlight bool
	// usageFresh says a reading landed that the scene has not taken, and
	// usageLanded that a probe came back. Item 79.
	usageFresh  bool
	usageLanded bool
	// summarise generates a session's summary; generating is the runs
	// still out and generatedDone what has come back for the frame loop
	// to hand over. Item 73. Both are read and written under mu.
	summarise     func(ctx context.Context, id string) (string, error)
	generating    map[string]bool
	generatedDone []generatedResult
	clicked       ebiten.Key
	hidden        bool
	// live is whether anyone was watching the window last tick, so the
	// tick is only changed when that changes. ticks counts Update calls
	// and painted the tick the last frame was painted for, so a repeat
	// of a frame already on screen can be skipped.
	live    bool
	tps     int
	ticks   uint64
	painted uint64
	// static is the city under the traffic, composed once and blitted
	// until the snapshot, the camera, the heading or the view changes.
	static staticLayer
	// carFadeLayer is where a hovered car's fader draws what it fades
	// (item 108).
	carFadeLayer *ebiten.Image

	settings     ui.Settings
	settingsOpen bool
	// carrying is the selected building being dragged to a group's line
	// (item 87).
	carrying *city.Building
	// firstRunAsked is whether this launch has shown the first run.
	firstRunAsked bool
	// groupSource is what New session guesses from: the history
	// detected once, and the recently used directories.
	groupSource func() (*group.Guesser, []string)
	// liveliness is how much the city moves, and reduced is its
	// stillest level, at which transitions cross-fade. motion is the
	// clocks the map animates to. Item 89.
	liveliness city.Liveliness
	reduced    bool
	motion     motionClock
	// signage is which of phase 21 item 38's title treatments is drawn.
	signage city.Signage
	apply   func(ui.Setting) error
	stats   statsHooks

	// pinned is the selection's card as last drawn, for clicks on it.
	pinned      ui.Card
	pinnedKinds []city.ActionKind

	sidebar       bool
	sidebarLayout ui.Sidebar

	breakdown       bool
	window          city.Window
	breakdownLayout ui.BreakdownPanel

	// The plant's long ranges (item 116, part F): whether one is shown,
	// which, with its districts and comparison, the district chooser, the
	// reader that fetches them off the frame, and the layouts last drawn.
	rangeOn      bool
	rangeSel     ui.RangeSelection
	rangeChooser ui.Chooser
	ranges       *rangeSource
	rangeClock   func() time.Time
	rangesLayout ui.RangesPanel
	rowLayout    ui.WindowRow

	// timeline is the event list on t; away is the list shown when the
	// window comes back into focus, or nil for the whole log.
	timeline       bool
	away           []events.Event
	cursor         int
	timelineLayout ui.Timeline
	focused        bool
	blurredAt      time.Time
	// awaySince is a moment the away list is owed for — the last time
	// the window was seen, or when it lost focus — once the first batch
	// of events has arrived; pendingEvents is that batch, queued by the
	// feed for the game loop.
	awaySince      time.Time
	eventsArrived  bool
	pendingEvents  []events.Event
	pendingBatches int

	// hits are the sprites drawn last frame, front last, with what each
	// stands for; the pointer is tested against them after the map's
	// footprints, so a tall building or tower is hovered by its body.
	hits      []spriteHit
	frameHits []spriteHit
	// planLayer is where the City Plan is drawn while it cross-fades
	// with the map (item 106).
	planLayer *ebiten.Image
	// streamLaid is the stream as last drawn, so a click lands on the
	// entry that is there (item 88).
	streamLaid ui.Stream
	// streamAsked is a show the stream's key made this run in a window
	// too narrow for the column (item 100).
	streamAsked bool
	// streamSkip is how many of the newest entries the panel is scrolled
	// past, streamSeen how many entries there were when it was last laid
	// out, streamWheel the part of a notch not yet scrolled, and
	// streamDrag whether its bar is held (item 100).
	streamSkip  int
	streamSeen  int
	streamWheel float64
	// minimapBox is where the minimap was drawn, empty when it was not:
	// a cover the key list lays itself out around (item 127).
	minimapBox city.Rect
	streamDrag bool
	// captions are explain mode's, or a first sighting's, as last drawn;
	// scriptedHeld is the keys a script holds down (item 88).
	captions     []ui.Caption
	scriptedHeld map[ebiten.Key]bool
	// quit is the two-press guard on Escape.
	quit confirm
	// overlays is the stack of panels over the city; the top one takes
	// every key and click. Item 82. finderRect and finderCaret are where
	// the finder was drawn, for clicks and for the input method's
	// candidate window; ime is whether there is a window to type into.
	overlays    overlays
	finderRect  city.Rect
	finderCaret city.Rect
	ime         bool
	// search finds sessions by what was said in them, from the daemon
	// or an index the city builds; dispatch runs it off the frame.
	// readConversation reads a page of a session's conversation for the
	// reading pane, and readerRect is where it was last drawn. Item 83.
	readConversation func(id string, skip, count int) (claude.Conversation, error)
	// readPrompts is what Claude Code's prompt history kept for a session
	// whose transcript is gone.
	readPrompts func(id string) ([]claude.Prompt, error)
	readerRect  city.Rect
	search      func(find.Query) ([]find.Hit, error)
	dispatch    func(func())
	// findAtOnce searches without waiting for typing to pause, for
	// scripted frames.
	findAtOnce bool
	// query is the map's filter; scriptedRune is a typed character from
	// --keys; scriptedShift, scriptedCtrl, scriptedAlt and scriptedCmd
	// are a modifier the
	// script holds with this frame's key.
	query        string
	scriptedCtrl bool
	scriptedAlt  bool
	scriptedCmd  bool
	// chords are the finder's, as configured; nil is the defaults for
	// this system. Item 82.
	chords *config.Chords
	// requests are what other commands ask of this, the one city, and
	// window is how it comes forward when asked. Item 82.
	requests <-chan single.Request
	win      windowSeam
	// shortcut is the global shortcut, nil where the desktop has none.
	// Item 84.
	shortcut shortcutSeam
	// desktopErr says why the desktop has no shortcut, and
	// recordingShortcut and pendingShortcut are the settings row
	// recording one. Item 84.
	desktopErr        error
	recordingShortcut bool
	pendingShortcut   *config.Chord
	scriptedRune      rune
	scriptedShift     bool
	dragging          bool
	dragFrom          city.Point
	// moving is a district being dragged to another slot, picked up at
	// pressAt; a press that never leaves its spot is a click. Item 85.
	moving  bool
	pressAt city.Point
	// journalHours is how long away the map must have been for the
	// journal to open by itself, and journalChecked whether the first
	// snapshot has been looked at for it; lastCloseUp reads the last
	// close-up and lastReply the last paragraph of a session's last
	// reply, both read when the journal opens (item 86).
	journalHours   int
	journalChecked bool
	lastCloseUp    func() (state.CloseUp, error)
	lastReply      func(id string) string
	// weedFade is the alpha the lot's growth is drawn at this frame.
	weedFade float64
	// shoreLayer is the island and the sea, painted once per plan.
	shoreLayer *shoreLayer
	started    time.Time
	// driven is a city run a tick at a time by Drive; inject is this
	// tick's input and shown what the last frame drew. Item 91.
	driven bool
	inject *driveInput
	shown  Shown
	// mapLabels is the labels this frame asked for, laid out together
	// once the world is drawn, and shownLabels where they went (item
	// 102).
	mapLabels   []pendingLabel
	shownLabels []ShownLabel
	// drawing counts what this frame's map issues, as it is issued, and
	// drawnOrder is the last frame's count (item 75).
	drawing, drawnOrder drawOrder
	// sightingsQueued is whether this frame's first sightings were laid
	// out with the map's labels, so drawExplain leaves them be.
	sightingsQueued bool
	// labelFade is the layer faded labels are drawn into (item 102).
	labelFade *ebiten.Image
}

func NewGame(scene *city.Scene, actor Actor, theme ui.Theme, faces *faces, saveLayout func(*city.Layout), sprites *sprites) *Game {
	g := &Game{scene: scene, actor: actor, theme: theme, faces: faces, saveState: saveLayout, sprites: sprites, started: time.Now(), focused: true, ime: true}
	g.overlays.push(&lensPanel{})
	// A window that was closed owes the away list from when it was last
	// seen, the way a blurred one does from when it lost focus.
	g.awaySince = scene.Layout().Seen
	return g
}

func (g *Game) Offer(snapshot state.Snapshot) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pending = &snapshot
}

// AddEvents queues a batch from the feed for the game loop; an empty
// batch still counts as the log having arrived.
func (g *Game) AddEvents(fresh []events.Event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pendingEvents = append(g.pendingEvents, fresh...)
	g.pendingBatches++
}

// SetStatus puts a line above the key row for a few seconds.
func (g *Game) SetStatus(status string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.notice.Set(status, timeNow())
}

// Stop ends the loop at its next update, from any goroutine.
func (g *Game) Stop() { g.stopping.Store(true) }

func (g *Game) Update() error {
	g.glHeld.Do(g.holdTheGL)
	if g.stopping.Load() {
		if g.frames != nil {
			_ = g.frames.close()
			g.frames = nil
		}
		return ebiten.Termination
	}
	if !g.startTick() {
		return nil
	}
	frames.tick()
	g.mu.Lock()
	pending := g.pending
	g.pending = nil
	fresh, batches := g.pendingEvents, g.pendingBatches
	g.pendingEvents, g.pendingBatches = nil, 0
	g.mu.Unlock()
	for _, done := range g.takeGenerated() {
		g.scene.SetGenerated(done.id, done.generated)
	}
	if batches > 0 {
		g.scene.AddEvents(fresh)
		g.eventsArrived = true
	}
	if pending != nil {
		g.scene.SetSnapshot(*pending)
		g.maybeJournal()
		g.followUsage()
		if title := windowTitle + " — " + g.scene.City().Summary().Headline(); title != g.shownTitle {
			g.shownTitle = title
			ebiten.SetWindowTitle(title)
		}
		if (g.screenshot != "" || g.record != "") && g.shotFrames == 0 {
			g.shotFrames = 1
		}
	}
	g.followUsageInScene()
	g.scripted, g.scriptedRune, g.scriptedShift, g.scriptedCtrl = -1, 0, false, false
	g.scriptedAlt, g.scriptedCmd = false, false
	g.clicked = -1
	g.play = playFrame{}
	if g.player != nil && g.record != "" && g.shotFrames > 0 {
		g.play = g.player.step(float64(g.recorded)/liveTPS, g.resolveTarget)
		if key, shift, ok := scriptedKey(g.play.key); ok && g.play.key != "" {
			g.scripted, g.scriptedShift = key, shift
		}
		if g.play.press {
			g.clickedAt = float64(g.recorded) / liveTPS
		}
		if g.play.statsPrompt != "" {
			g.playStatsPrompt(g.play.statsPrompt)
		}
	}
	if g.record != "" && g.shotFrames > 0 {
		// A key every two seconds, then run the clock out.
		if len(g.script) > 0 && g.recorded%60 == 30 {
			g.pressScripted(g.script[0])
			g.script = g.script[1:]
		}
	} else if g.shotFrames > 0 && len(g.script) > 0 {
		g.pressScripted(g.script[0])
		g.script = g.script[1:]
		g.shotFrames = 1
	}
	g.injectInput()
	if g.shotDone {
		if g.frames != nil {
			if err := g.frames.close(); err != nil && g.shotErr == nil {
				g.shotErr = err
			}
			g.frames = nil
		}
		if g.shotErr != nil {
			return g.shotErr
		}
		return ebiten.Termination
	}

	g.scene.SetInstant(g.reduced || g.screenshot != "")
	g.scene.SetReducedMotion(g.reduced)
	g.scene.SetPlanFade(city.PlanFade(g.liveliness))
	g.scene.Animate(1.0 / 30)
	if g.screenshot == "" && !g.driven {
		g.watchFocus()
	}
	cursor := g.notePointer()

	if _, wheel := ebiten.Wheel(); wheel != 0 && !g.overlayWheel(cursor, wheel) && !g.wheelStream(cursor, wheel) {
		g.scene.Wheel(cursor, wheel)
	}
	if g.play.wheel != 0 && !g.overlayWheel(cursor, g.play.wheel) && !g.wheelStream(cursor, g.play.wheel) {
		g.scene.Wheel(cursor, g.play.wheel)
	}
	if g.streamDrag {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || g.play.shown && !g.play.release {
			g.dragStream(cursor)
		} else {
			g.streamDrag = false
		}
	}
	if g.play.pan != (city.Point{}) {
		g.scene.Pan(g.play.pan)
	}
	// The split between press and release is load-bearing, and it is
	// luck rather than design, so it is written down before someone
	// spends it. Card buttons are hit on press and the selection is made
	// on release, which is the only reason a single click cannot press a
	// button on the card it just raised — and that card carries stop two
	// along from attach. Move either to the other event and a click
	// becomes able to stop a session.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || g.play.press {
		if !g.clickOverlay(cursor) && !g.clickSettings(cursor) && !g.clickSearch(cursor) && !g.clickFooter(cursor) && !g.clickViewKey(cursor) && !g.clickTimeline(cursor) && !g.clickBreakdown(cursor) && !g.clickCard(cursor) && !g.clickStream(cursor) && !g.clickSidebar(cursor) && !g.stripClick(cursor) {
			// A press on a district's floor or sign picks the district up;
			// anywhere else, or with Shift or Alt held anywhere, it pans.
			if carries(g.scene.Hover(), g.scene.Selected()) {
				g.carrying = g.scene.Selected()
			} else if g.scene.PressDistrict(cursor, panHeld()) {
				g.moving, g.pressAt = true, cursor
			} else {
				g.dragging, g.dragFrom = true, cursor
			}
		}
	}
	if g.moving && (ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || g.play.shown) {
		g.scene.DragDistrict(cursor)
	}
	if (inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) || g.play.release) && g.moving {
		g.moving = false
		if math.Hypot(cursor.X-g.pressAt.X, cursor.Y-g.pressAt.Y) > 3 {
			if g.scene.DropDistrict(cursor) && g.saveState != nil {
				g.saveState(g.scene.Layout())
			}
		} else {
			g.scene.CancelDistrictDrag()
			g.act(g.scene.Click(cursor))
		}
	}
	if g.dragging && (ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || g.play.shown) {
		g.scene.Pan(cursor.Sub(g.dragFrom))
		g.dragFrom = cursor
	}
	if (inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) || g.play.release) && g.carrying != nil {
		if drop := dropAction(g.carrying, g.scene.Hover()); drop.Kind == city.ActionMove {
			g.move(drop.SessionID, drop.Group)
		}
		g.carrying = nil
	}
	if (inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) || g.play.release) && g.dragging {
		wasDrag := math.Hypot(cursor.X-g.dragFrom.X, cursor.Y-g.dragFrom.Y) > 3
		g.dragging = false
		if !wasDrag {
			g.clickLandmark(g.scene.Hover().Landmark)
			g.act(g.scene.Click(cursor))
		}
	}
	g.maybeFirstRun()
	g.maybeStatsPrompt()
	return g.handleKeys()
}

// capture writes the frame just drawn to the screenshot path. It waits for
// the second frame after the first snapshot so the fit has settled.
// resolveTarget is where a scripted pointer aims, or a scripted pan
// brings to the middle: a session's building
// by title or id, a district by "district:<name>", a car by
// "car:<from>-><to>", or the power plant.
func (g *Game) resolveTarget(target string) (city.Point, bool) {
	c := g.scene.City()
	cam := g.scene.Camera()
	if target == "plant" && c.Plant.Rect.Area() > 0 {
		return cam.WorldToScreen(c.Plant.Rect.Center()), true
	}
	if target == "needs-you line" {
		g.mu.Lock()
		line := g.footerBar.WaitingRect
		g.mu.Unlock()
		return line.Center(), line.Area() > 0
	}
	if id, ok := strings.CutPrefix(target, "term:"); ok {
		r := g.referenceOpen()
		if r == nil {
			return city.Point{}, false
		}
		g.mu.Lock()
		at, ok := r.laid.TermRect(id)
		g.mu.Unlock()
		return at.Center(), ok
	}
	if pair, ok := strings.CutPrefix(target, "car:"); ok {
		return g.carTarget(c, cam, pair)
	}
	if name, ok := strings.CutPrefix(target, "district:"); ok {
		if g.scene.CityPlanOpen() {
			for _, b := range g.scene.CityPlan().Badges {
				if filepath.Base(b.District.Root) == name {
					return b.At, true
				}
			}
			return city.Point{}, false
		}
		for _, d := range c.Districts {
			if filepath.Base(d.Root) == name {
				return cam.WorldToScreen(d.Rect.Center()), true
			}
		}
		return city.Point{}, false
	}
	for _, b := range c.Buildings() {
		if b.Session.Title == target || (len(target) >= 8 && strings.HasPrefix(b.Session.ID, target)) {
			return cam.WorldToScreen(b.Rect.Center()), true
		}
	}
	return city.Point{}, false
}

// just reports a key pressed this frame, by hand or by the script.
func (g *Game) just(key ebiten.Key) bool {
	return g.scripted == key || g.clicked == key || inpututil.IsKeyJustPressed(key)
}

// pointer is where the pointer is: the mouse, or the point a --hover
// or a script parked it at.
func (g *Game) pointer() city.Point {
	if g.play.shown {
		return g.play.pointer
	}
	if g.hoverSet {
		return g.hover
	}
	x, y := cursorPosition()
	return city.Point{X: float64(x), Y: float64(y)}
}

// cursorPosition is the system cursor, swapped out in tests.
var cursorPosition = ebiten.CursorPosition

// notePointer tells the scene what the pointer is over, and returns
// where it is. A recording or a screenshot has no mouse: the system
// cursor sits wherever the display put it, in the middle of the screen
// under xvfb, so until the script shows a pointer or --hover parks one,
// it is over nothing (item 115).
func (g *Game) notePointer() city.Point {
	cursor := g.pointer()
	if (g.record != "" || g.screenshot != "") && !g.play.shown && !g.hoverSet {
		g.scene.PointerAway()
		return cursor
	}
	g.scene.PointerMove(cursor)
	g.hoverSprites(cursor)
	return cursor
}

// keyByName finds an Ebitengine key by its name, case-insensitively:
// "equal", "b", "ArrowLeft".
func keyByName(name string) (ebiten.Key, bool) {
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if strings.EqualFold(k.String(), name) {
			return k, true
		}
	}
	return -1, false
}

// scriptedKey reads one entry of --keys: a key's own name, or a typed
// character that needs shift, such as "?" for help.
func scriptedKey(name string) (key ebiten.Key, shift bool, ok bool) {
	if rest, cut := strings.CutPrefix(name, "hold+"); cut {
		name = rest
	} else if rest, cut := strings.CutPrefix(name, "release+"); cut {
		name = rest
	}
	if name == "?" {
		return ebiten.KeySlash, true, true
	}
	key, ok = keyByName(name)
	return key, false, ok
}

// pressScripted presses one key of a --keys script: a key by name,
// "ctrl+" and a key to hold Ctrl with it, or a character, which is
// typed when the finder is open.
func (g *Game) pressScripted(name string) {
	for prefix, down := range map[string]bool{"hold+": true, "release+": false} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			if key, _, ok := scriptedKey(rest); ok {
				if g.scriptedHeld == nil {
					g.scriptedHeld = map[ebiten.Key]bool{}
				}
				g.scriptedHeld[key] = down
			}
			return
		}
	}
	for held := true; held; {
		held = false
		for prefix, mod := range map[string]*bool{"ctrl+": &g.scriptedCtrl, "alt+": &g.scriptedAlt, "cmd+": &g.scriptedCmd, "shift+": &g.scriptedShift} {
			if rest, ok := strings.CutPrefix(name, prefix); ok {
				name, *mod, held = rest, true, true
			}
		}
	}
	if key, shift, ok := scriptedKey(name); ok {
		g.scripted, g.scriptedShift = key, shift || g.scriptedShift
	}
	typing := g.finding() || g.newSession() != nil || (g.triage() != nil && g.triage().snoozing != nil)
	if r := []rune(name); len(r) == 1 && typing && !g.scriptedCtrl && !g.scriptedAlt && !g.scriptedCmd {
		g.scriptedRune = r[0]
	}
}

// shifted reports whether shift is held, by hand or by the script.
func (g *Game) shifted() bool {
	return g.scriptedShift || ebiten.IsKeyPressed(ebiten.KeyShift)
}

func (g *Game) capture(screen *ebiten.Image) {
	if g.record != "" && g.shotFrames > 0 && !g.shotDone {
		every := max(g.recordEvery, 1)
		if g.recorded%every == 0 {
			path := filepath.Join(g.record, fmt.Sprintf("frame-%05d.png", g.recorded/every))
			if g.frames == nil {
				g.frames = newFrameWriter(frameWorkers)
			}
			if err := g.frames.failed(); err != nil {
				g.shotErr, g.shotDone = err, true
				return
			}
			g.frames.write(path, frame(screen))
		}
		if g.recorded == 0 {
			g.clipFrom = g.ticks
		}
		g.clipTo = g.ticks
		g.recorded++
		if g.recorded >= g.recordFrames {
			g.shotDone = true
		}
		return
	}
	if g.snap != "" {
		if err := writePNG(g.snap, frame(screen)); err != nil {
			g.SetStatus(err.Error())
		} else {
			g.SetStatus("saved " + g.snap)
		}
		g.snap = ""
	}
	if g.screenshot == "" || g.shotFrames == 0 || g.shotDone {
		return
	}
	g.shotFrames++
	if !g.shotReady() {
		return
	}
	g.shotErr = writePNG(g.screenshot, frame(screen))
	g.shotDone = true
}

// shotReady is whether the screenshot can be taken: its script is done
// and the frame has settled. Draws can outrun updates on a real X
// server, so counting frames alone took the shot before the last keys.
func (g *Game) shotReady() bool {
	return len(g.script) == 0 && g.shotFrames >= 3
}

func frame(screen *ebiten.Image) image.Image {
	b := screen.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	screen.ReadPixels(img.Pix)
	return img
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (g *Game) Draw(screen *ebiten.Image) {
	if !g.idle() {
		// Nobody can see this frame. Holding here is what keeps the
		// rate down: Draw is called once per display refresh, and an
		// unmapped window has no refresh to wait for.
		time.Sleep(idleSleep)
		return
	}
	if !g.startPaint() {
		// This frame would repaint what is already on the glass. The
		// screen is not cleared between frames, so leaving it is enough.
		return
	}
	defer g.capture(screen)
	defer g.drawPointer(screen)
	c := g.scene.City()
	cam := g.scene.Camera()
	hover := g.scene.Hover()
	selected := g.scene.Selected()
	// Worked out once a frame rather than once an object: it walks every
	// building, and every building asks it.
	g.lit = g.scene.Highlight()
	labels := g.labelsVisible()
	detailed := g.scene.Detailed()
	bounds := screen.Bounds()
	width, height := float64(bounds.Dx()), float64(bounds.Dy())
	seconds := g.clock()

	if g.driven {
		defer g.noteShown()
	}
	if cam.Projection == city.Isometric {
		shown := g.scene.PlanShown()
		if shown < 1 {
			g.drawIso(screen, c, cam, hover, selected, width, height, seconds)
			g.districtGhost(screen, cam)
			if g.carrying != nil {
				x, y := ebiten.CursorPosition()
				g.drawCarry(screen, cam, city.Point{X: float64(x), Y: float64(y)})
			}
		}
		g.drawCityPlan(screen, width, height, shown)
		g.overlay(screen, width, height)
		return
	}

	g.ground(screen, cam, c, width, height, detailed)
	g.flatCells(screen, cam, c)
	g.drivewayPads(screen, c, cam, true)
	g.overgrowth(screen, cam, true)
	if len(c.Corridors) > 0 {
		g.streetSigns(screen, c, cam, hover, labels)
	} else {
		g.roadLines(screen, c, cam, hover, labels)
	}
	g.beams(screen, c, cam, hover)
	g.powerLineStrokes(screen, c, cam, hover)

	for _, d := range c.Districts {
		g.district(screen, cam, d, hover.District == d, c.Night, detailed)
	}
	for _, d := range c.Districts {
		// In the isometric city the project's name is a monument sign
		// standing in its plaza, drawn with the scene. The floor plate
		// is the top-down city's version of the same thing. Bug 44
		// rejected having both, and rejected the plate as chrome.
		if g.districtLabelVisible(d) && cam.Projection != city.Isometric {
			g.floorLabel(screen, g.districtLabelAt(d), d.Name, colorText)
		}
		for _, b := range d.Buildings {
			g.building(screen, cam, b, b == selected, detailed, seconds)
		}
	}
	if g.titlesVisible() {
		for _, b := range c.Buildings() {
			if b.Blinds && hover.Building != b {
				continue
			}
			g.title(screen, cam, b)
		}
	}
	g.landmarks(screen, cam, labels)
	g.districtGhost(screen, cam)
	g.overlay(screen, width, height)
}

// overlay is the chrome and, over it, the help; h hides the lot.
func (g *Game) overlay(screen *ebiten.Image, width, height float64) {
	if g.hidden {
		g.scene.SetTopChrome(0)
		g.scene.SetBottomChrome(0)
		g.drawOverlays(screen, width, height, false)
		return
	}
	g.drawExplain(screen, width, height)
	g.chrome(screen, width, height)
	g.drawOverlays(screen, width, height, true)
	if g.viewKey {
		g.drawViewKey(screen, width, height)
	}
	if g.settingsOpen {
		g.drawSettings(screen, width, height)
	}
	if g.breakdown {
		g.drawBreakdown(screen, width, height)
	}
	if g.timeline {
		g.drawTimeline(screen, width, height)
	}
	g.drawOverlays(screen, width, height, false)
}

// spriteHit is a drawn sprite's screen rect and what it stands for.
type spriteHit struct {
	rect city.Rect
	hit  city.Hit
}

// hoverSprites lets a sprite's body take the hover when the map's
// footprints found only ground under the pointer.
func (g *Game) hoverSprites(cursor city.Point) {
	g.mu.Lock()
	hits := g.hits
	g.mu.Unlock()
	if hit, ok := pickHit(g.scene.Hover(), hits, cursor); ok {
		g.scene.SetHover(hit)
	}
}

// pickHit is what the pointer has landed on, given where the world says
// it is and every sprite drawn this frame.
//
// Sprites are searched back to front, so the thing drawn last — the
// thing on top — wins. A sprite normally only gets a say when the world
// hover is open ground, because a building already knows its own
// footprint; the exception is the movers. A drone works on a building's
// faces and roof, a flag stands on one and smoke rises off it, so all
// three are inside or above the footprint of the building they belong
// to. Without letting them override, they are hoverable in principle and
// unreachable in practice, which is the whole of what this was meant to
// fix.
func pickHit(world city.Hit, hits []spriteHit, cursor city.Point) (city.Hit, bool) {
	for i := len(hits) - 1; i >= 0; i-- {
		if !hits[i].rect.Contains(cursor) {
			continue
		}
		if world.Ground() || hits[i].hit.Mover() {
			return hits[i].hit, true
		}
		return city.Hit{}, false
	}
	return city.Hit{}, false
}

// timeNow is the wall clock, one seam for the keys that stamp files.
var timeNow = time.Now

// clock is the time ambient motion animates to, run at the liveliness's
// pace: at still it stands where it is, so every mover stays a marker.
func (g *Game) clock() float64 {
	g.stepMotion()
	return g.motion.ambient
}

// attention is the time the states that want you pulse to. At still,
// attention is colour only: the clock stands at the point where the pulse
// is at full colour (item 89).
func (g *Game) attention() float64 {
	if g.reduced {
		return pulsePeriod / 4
	}
	g.stepMotion()
	return g.motion.attention
}

func (g *Game) stepMotion() {
	if g.motion.last.IsZero() {
		g.motion = newMotionClock(g.started)
	}
	g.motion.step(g.now(), g.liveliness)
}

// setLiveliness changes how much the city moves from here on.
func (g *Game) setLiveliness(l city.Liveliness) {
	g.stepMotion()
	g.liveliness, g.reduced = l, l.CrossFades()
}

// now is the time the city animates to. While recording it is counted
// in ticks, so a clip plays at the speed the city would have run
// however slowly the frames were drawn: on software GL a tick takes
// three or four times as long as it should, and wall-clock animation
// ran that much too fast on film.
func (g *Game) now() time.Time {
	if g.record != "" || g.driven {
		return g.started.Add(time.Duration(g.ticks) * time.Second / liveTPS)
	}
	return timeNow()
}

func (g *Game) labelsVisible() bool { return !g.hidden && g.scene.LabelsVisible() }
func (g *Game) titlesVisible() bool { return !g.hidden && g.scene.TitlesVisible() }
func (g *Game) districtLabelVisible(d *city.District) bool {
	return !g.hidden && g.scene.DistrictLabelVisible(d)
}

// powerLineStrokes is the top-down view's power lines: plain strokes.
func (g *Game) powerLineStrokes(screen *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit) {
	if !g.shows(city.NetworkWires) {
		return
	}
	lines := c.PowerLines()
	for _, line := range lines {
		cached, fresh := colorLineCached, colorLineFresh
		if t := g.scene.WireTint(line, lines); t.Known {
			cached, fresh = ui.Sequential.At(t.Value), ui.Sequential.At(t.Value)
		}
		g.line(screen, cam, line.From, line.To, lineWidth(line.Cached, 1, 3), cached)
		g.line(screen, cam, line.From, line.To, lineWidth(line.Fresh, 1, 5), fresh)
		if hover.Line != nil && hover.Line.Building == line.Building {
			g.line(screen, cam, line.From, line.To, 2, colorHighlight)
		}
	}
}

func (g *Game) roadLines(screen *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit, labels bool) {
	for i := range c.Roads {
		road := &c.Roads[i]
		g.line(screen, cam, road.A, road.B, 8, colorKerb)
		g.line(screen, cam, road.A, road.B, 6, colorRoad)
		if hover.Road == road || g.lit.HasRoad(road) {
			g.line(screen, cam, road.A, road.B, 2, colorHighlight)
		}
		if labels {
			mid := city.Point{X: (road.A.X + road.B.X) / 2, Y: (road.A.Y + road.B.Y) / 2}
			g.floorLabel(screen, cam.WorldToScreen(mid).Add(city.Point{X: g.theme.Px(4), Y: -g.lineHeight() + g.theme.Px(2)}), road.Label(), colorDim)
		}
	}
}

func (g *Game) beams(screen *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit) {
	if !g.shows(city.NetworkBeams) {
		return
	}
	beams := c.Beams()
	for _, beam := range beams {
		// Servers paints the carrier as well as what it runs to: a beam
		// by the calls it has carried, so the thick line and the bright
		// one say the same thing rather than only one of them.
		col := colorBeam
		if t := g.scene.BeamTint(beam, beams); t.Known {
			col = ui.Sequential.At(t.Value)
		}
		g.line(screen, cam, beam.From, beam.To, lineWidth(float64(beam.Calls)*20_000, 1, 3), col)
		if hover.Beam != nil && hover.Beam.Building == beam.Building && hover.Beam.Tower == beam.Tower {
			g.line(screen, cam, beam.From, beam.To, 2, colorHighlight)
		}
	}
}

func formatTitle(title string) string {
	return format.Clip(title, titleChars)
}

// title writes a building's name under it at a fixed size.
func (g *Game) title(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	name := formatTitle(b.Card(g.scene.City().Time).Title)
	w, _ := g.measure(name)
	at := cam.WorldToScreen(city.Point{X: b.Rect.Center().X, Y: b.Rect.Max.Y}).Add(city.Point{X: -w / 2, Y: g.theme.Px(4)})
	g.floorLabel(screen, at, name, colorText)
}

// districtLabelAt is where a district's name plate sits for this face.
func (g *Game) districtLabelAt(d *city.District) city.Point {
	w, h := g.measure(d.Name)
	return g.scene.DistrictLabelAt(d, h, w)
}

func (g *Game) building(screen *ebiten.Image, cam *city.Camera, b *city.Building, selected, detailed bool, seconds float64) {
	if !detailed {
		g.block(screen, cam, b, seconds)
		if selected {
			g.outline(screen, cam, b.Rect, colorSelected)
		}
		return
	}
	body := colorBuilding
	switch {
	case b.Blinds:
		body = colorBoarded
	case b.Vacant:
		body = colorMapVacant
	case b.Pulse:
		body = pulse(colorNeedsYou, g.attention())
	case b.Session.State == state.Unattended:
		body = colorUnattended
	case b.Lit:
		body = colorLit
	}
	body = g.flatBody(b, body)
	if g.sprites != nil && b.Blinds {
		g.shack(screen, cam, b)
		if selected {
			g.outline(screen, cam, b.Rect, colorSelected)
		}
		return
	}
	if g.sprites != nil {
		g.house(screen, cam, b, body)
	} else {
		g.rect(screen, cam, b.Rect, body)
		g.windows(screen, cam, b)
	}

	if b.Fill > 0 {
		if g.sprites != nil {
			gauge := city.RectAt(b.Rect.Max.X+2, b.Rect.Min.Y, 5, b.Rect.Height())
			g.rect(screen, cam, gauge, colorGaugeBack)
			height := gauge.Height() * b.Fill
			g.rect(screen, cam, city.Rect{Min: city.Point{X: gauge.Min.X, Y: gauge.Max.Y - height}, Max: gauge.Max}, gaugeColor(b.Fill))
		} else {
			height := b.Rect.Height() * b.Fill
			fillRect := city.Rect{
				Min: city.Point{X: b.Rect.Min.X, Y: b.Rect.Max.Y - height},
				Max: b.Rect.Max,
			}
			g.rect(screen, cam, fillRect, colorFill)
		}
	}
	for i := 0; g.shows(city.NetworkCranes) && i < b.Cranes; i++ {
		if g.sprites != nil {
			x := b.Rect.Min.X + 2 + float64(i)*14
			g.drawTile(screen, cam, g.sprites.factory.tile(factoryChain[0], factoryChain[1]), city.RectAt(x, b.Rect.Min.Y-26, 14, 14), nil)
			g.drawTile(screen, cam, g.sprites.factory.tile(factoryHook[0], factoryHook[1]), city.RectAt(x, b.Rect.Min.Y-13, 14, 14), nil)
			continue
		}
		crane := city.RectAt(b.Rect.Min.X+4+float64(i)*8, b.Rect.Min.Y-6, 5, 10)
		g.rect(screen, cam, crane, colorCrane)
	}
	if g.sprites != nil && b.Session.State == state.Working && b.Session.Tool != "" {
		g.drawTile(screen, cam, g.sprites.factory.tile(factoryWorker[0], factoryWorker[1]),
			city.RectAt(b.Rect.Max.X-16, b.Rect.Max.Y-2, 16, 16), nil)
	}
	if b.Flags > 0 {
		pole := city.RectAt(b.Rect.Max.X-6, b.Rect.Min.Y-14, 1.5, 14)
		flag := city.RectAt(b.Rect.Max.X-6, b.Rect.Min.Y-14, 8, 5)
		g.rect(screen, cam, pole, colorPole)
		g.rect(screen, cam, flag, colorFlag)
	}
	if b.Smoke > 0 {
		t := seconds
		for i := 0; i < min(b.Smoke, 3); i++ {
			phase := math.Mod(t*0.4+float64(i)*0.33, 1)
			centre := city.Point{X: b.Rect.Min.X + 12 + float64(i)*10 + 4*math.Sin(phase*6), Y: b.Rect.Min.Y - 4 - phase*18}
			g.circle(screen, cam, centre, 3+phase*3, colorSmoke)
		}
	}
	if selected {
		g.outline(screen, cam, b.Rect, colorSelected)
	}
}

func (g *Game) district(screen *ebiten.Image, cam *city.Camera, d *city.District, hovered, night, detailed bool) {
	if g.sprites == nil || !detailed {
		fill := colorMapDistrict
		if hovered {
			fill = colorMapDistHi
		}
		g.rect(screen, cam, d.Rect, g.flat(fill))
		g.stroke(screen, cam, d.Rect, 1, g.flat(colorKerb))
		return
	}
	scale := &ebiten.ColorScale{}
	scale.SetR(0.55)
	scale.SetG(0.55)
	scale.SetB(0.6)
	if hovered {
		scale.SetR(0.75)
		scale.SetG(0.75)
		scale.SetB(0.8)
	}
	if night {
		scale.SetR(scale.R() * 0.6)
		scale.SetG(scale.G() * 0.6)
		scale.SetB(scale.B() * 0.75)
	}
	cols := int(math.Ceil(d.Rect.Width() / city.Tile))
	rows := int(math.Ceil(d.Rect.Height() / city.Tile))
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			pick := modernPave[(row+col)%2]
			r := city.RectAt(d.Rect.Min.X+float64(col)*city.Tile, d.Rect.Min.Y+float64(row)*city.Tile,
				math.Min(city.Tile, d.Rect.Max.X-d.Rect.Min.X-float64(col)*city.Tile), math.Min(city.Tile, d.Rect.Max.Y-d.Rect.Min.Y-float64(row)*city.Tile))
			g.drawTile(screen, cam, g.sprites.modern.tile(pick[0], pick[1]), r, scale)
		}
	}
	g.stroke(screen, cam, d.Rect, 2, colorKerb)
}

func (g *Game) house(screen *ebiten.Image, cam *city.Camera, b *city.Building, body color.NRGBA) {
	roof := townRoofGrey
	if b.Pulse {
		roof = townRoofOrange
	}
	walls := townWallWood
	if b.Blinds {
		walls = townWallStone
	}
	window := townWindowDark
	if b.Lit {
		window = townWindowLit
	}
	var scale *ebiten.ColorScale
	if b.Blinds {
		scale = &ebiten.ColorScale{}
		scale.SetR(0.6)
		scale.SetG(0.6)
		scale.SetB(0.65)
	} else if b.Pulse {
		scale = &ebiten.ColorScale{}
		f := float32(body.R) / float32(colorNeedsYou.R)
		scale.SetR(f)
		scale.SetG(f)
		scale.SetB(f)
	}
	// A house is the one thing in the flat path a view has something to
	// say about, so it takes the view's colour where everything else
	// recedes. nil asks drawTileTinted for the recede instead, which is
	// what a building the view cannot place should get.
	var tint *ebiten.ColorScale
	if g.viewing() && !g.unlit(b) {
		if c, ok := g.viewTint(b); ok {
			tint = viewScale(c)
		}
	}
	draw := func(t *ebiten.Image, r city.Rect) {
		g.drawTileTinted(screen, cam, t, r, scale, tint)
	}
	for col := 0; col < 3; col++ {
		draw(g.sprites.town.tile(roof[col][0], roof[col][1]), cell(b.Rect, 3, 3, col, 0))
	}
	for col := 0; col < 3; col++ {
		t := walls[col]
		if col != 1 {
			t = window
		}
		draw(g.sprites.town.tile(t[0], t[1]), cell(b.Rect, 3, 3, col, 1))
	}
	for col := 0; col < 3; col++ {
		draw(g.sprites.town.tile(walls[col][0], walls[col][1]), cell(b.Rect, 3, 3, col, 2))
	}
}

func (g *Game) windows(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	lit := colorWindowDark
	if b.Lit {
		lit = colorWindow
	}
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			w := city.RectAt(b.Rect.Min.X+8+float64(col)*15, b.Rect.Min.Y+8+float64(row)*13, 7, 6)
			g.rect(screen, cam, w, lit)
		}
	}
}

func (g *Game) landmarks(screen *ebiten.Image, cam *city.Camera, labels bool) {
	c := g.scene.City()
	if c.Plant.Rect.Area() > 0 {
		if g.sprites != nil {
			glow := &ebiten.ColorScale{}
			f := 0.85 + 0.15*float32(math.Sin(g.clock()*2))
			glow.SetR(f)
			glow.SetG(f)
			glow.SetB(f)
			for col := 0; col < 3; col++ {
				g.drawTile(screen, cam, g.sprites.factory.tile(factoryMachine[col][0], factoryMachine[col][1]), cell(c.Plant.Rect, 3, 2, col, 0), glow)
				g.drawTile(screen, cam, g.sprites.factory.tile(factoryMachLow[col][0], factoryMachLow[col][1]), cell(c.Plant.Rect, 3, 2, col, 1), glow)
			}
		} else {
			g.rect(screen, cam, c.Plant.Rect, colorPlant)
			core := c.Plant.Rect.Inset(16)
			g.rect(screen, cam, core, pulse(colorPlantCore, g.clock()*0.5))
		}
		if labels {
			g.label(screen, cam.WorldToScreen(c.Plant.Rect.Min).Add(city.Point{X: g.theme.Px(4), Y: -g.lineHeight()}), "power plant", colorDim)
		}
	}
	for _, t := range c.Towers {
		fill := colorTower
		if t.Server.Calls > 0 {
			fill = colorTowerUsed
		}
		if g.sprites != nil {
			var scale *ebiten.ColorScale
			if t.Server.Calls == 0 {
				scale = &ebiten.ColorScale{}
				scale.SetR(0.5)
				scale.SetG(0.5)
				scale.SetB(0.55)
			}
			g.drawTile(screen, cam, g.sprites.factory.tile(factoryMast[0], factoryMast[1]), city.RectAt(t.Rect.Center().X-6, t.Rect.Min.Y-20, 12, 24), scale)
			g.drawTile(screen, cam, g.sprites.factory.tile(factoryGear[0], factoryGear[1]), t.Rect, scale)
		} else {
			mast := city.RectAt(t.Rect.Center().X-2, t.Rect.Min.Y-18, 4, 18)
			g.rect(screen, cam, mast, fill)
			g.rect(screen, cam, t.Rect, fill)
		}
		if labels {
			g.labelRight(screen, cam.WorldToScreen(city.Point{X: t.Rect.Min.X, Y: t.Rect.Center().Y}).Add(city.Point{X: -g.theme.Px(8), Y: -g.lineHeight() / 2}), t.Server.Name, colorDim)
		}
	}
	if c.Library.Rect.Area() > 0 {
		if g.sprites != nil {
			cols, rows := 3, 4
			for row := 0; row < rows; row++ {
				for col := 0; col < cols; col++ {
					t := townWallStone[col]
					if row == 0 {
						t = townRoofGrey[col]
					}
					g.drawTile(screen, cam, g.sprites.town.tile(t[0], t[1]), cell(c.Library.Rect, cols, rows, col, row), nil)
				}
			}
			g.drawTile(screen, cam, g.sprites.town.tile(townSign[0], townSign[1]), city.RectAt(c.Library.Rect.Min.X-18, c.Library.Rect.Max.Y-18, 16, 16), nil)
		} else {
			g.rect(screen, cam, c.Library.Rect, colorLibrary)
			for i := 0; i < 4; i++ {
				shelf := city.RectAt(c.Library.Rect.Min.X+8, c.Library.Rect.Min.Y+12+float64(i)*20, c.Library.Rect.Width()-16, 3)
				g.rect(screen, cam, shelf, colorDim)
			}
		}
		if labels {
			g.label(screen, cam.WorldToScreen(c.Library.Rect.Min).Add(city.Point{X: 0, Y: -g.lineHeight()}), "library", colorDim)
		}
	}
	if c.Hall.Rect.Area() > 0 {
		if g.sprites != nil {
			cols, rows := 4, 3
			for row := 0; row < rows; row++ {
				for col := 0; col < cols; col++ {
					t := townWallWood[col%len(townWallWood)]
					if row == 0 {
						t = townRoofOrange[col%len(townRoofOrange)]
					}
					g.drawTile(screen, cam, g.sprites.town.tile(t[0], t[1]), cell(c.Hall.Rect, cols, rows, col, row), nil)
				}
			}
		} else {
			g.rect(screen, cam, c.Hall.Rect, colorLibrary)
		}
		if labels {
			g.label(screen, cam.WorldToScreen(c.Hall.Rect.Min).Add(city.Point{X: 0, Y: -g.lineHeight()}), "city hall", colorDim)
		}
	}
}

func (g *Game) line(screen *ebiten.Image, cam *city.Camera, from, to city.Point, width float32, c color.NRGBA) {
	a := cam.WorldToScreen(from)
	b := cam.WorldToScreen(to)
	vector.StrokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), width*float32(cam.Zoom), c, true)
}

func (g *Game) circle(screen *ebiten.Image, cam *city.Camera, centre city.Point, radius float64, c color.NRGBA) {
	p := cam.WorldToScreen(centre)
	vector.FillCircle(screen, float32(p.X), float32(p.Y), float32(radius*cam.Zoom), c, true)
}

func lineWidth(perHour, min, max float64) float32 {
	if perHour <= 0 {
		return float32(min)
	}
	w := min + math.Log10(perHour/1000+1)
	return float32(math.Min(max, math.Max(min, w)))
}

func (g *Game) rect(screen *ebiten.Image, cam *city.Camera, r city.Rect, c color.NRGBA) {
	min := cam.WorldToScreen(r.Min)
	max := cam.WorldToScreen(r.Max)
	vector.FillRect(screen, float32(min.X), float32(min.Y), float32(max.X-min.X), float32(max.Y-min.Y), c, false)
}

func (g *Game) outline(screen *ebiten.Image, cam *city.Camera, r city.Rect, c color.NRGBA) {
	min := cam.WorldToScreen(r.Min)
	max := cam.WorldToScreen(r.Max)
	vector.StrokeRect(screen, float32(min.X), float32(min.Y), float32(max.X-min.X), float32(max.Y-min.Y), 2, c, false)
}

// text draws s with its top-left corner at a screen point, in one of the
// theme's four sizes.
func (g *Game) text(screen *ebiten.Image, at city.Point, s string, size ui.Size, c color.NRGBA) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(at.X, at.Y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, g.faces.Face(size), op)
}

// measure is the size of an in-world label, which is always set small.
func (g *Game) measure(s string) (float64, float64) {
	return g.faces.Measure(s, ui.Small)
}

// lineHeight is the height of one small line, the unit in-world labels
// and the footer stack by.
func (g *Game) lineHeight() float64 {
	_, h := g.measure("")
	return h
}

// label is an in-world label: small text straight onto the screen.
func (g *Game) label(screen *ebiten.Image, at city.Point, s string, c color.NRGBA) {
	g.text(screen, at, s, ui.Small, c)
}

func (g *Game) labelRight(screen *ebiten.Image, end city.Point, s string, c color.NRGBA) {
	width, _ := g.measure(s)
	g.label(screen, city.Point{X: end.X - width, Y: end.Y}, s, c)
}

// LayoutF sizes the frame in device pixels, so text and sprites are drawn
// at the display's own resolution instead of being scaled up afterwards.
func (g *Game) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	width, height := outsideWidth*g.theme.Scale, outsideHeight*g.theme.Scale
	g.scene.Resize(width, height)
	return width, height
}

// Layout is never called while LayoutF exists; it satisfies ebiten.Game.
func (g *Game) Layout(width, height int) (int, int) {
	w, h := g.LayoutF(float64(width), float64(height))
	return int(w), int(h)
}

func gaugeColor(fill float64) color.NRGBA {
	if fill < 0.8 {
		return colorGaugeLow
	}
	return colorGaugeHigh
}

func pulse(c color.NRGBA, seconds float64) color.NRGBA {
	t := 0.72 + 0.28*math.Sin(2*math.Pi*seconds/pulsePeriod)
	scale := func(v uint8) uint8 { return uint8(math.Round(float64(v) * t)) }
	return color.NRGBA{scale(c.R), scale(c.G), scale(c.B), c.A}
}

// KnownKey reports whether --keys or a script can press a key by this
// name, so a scenario's typo fails a test rather than a forty-minute run.
func KnownKey(name string) bool {
	_, _, ok := scriptedKey(name)
	return ok
}
