package city

import (
	"fmt"
	"math"
	"slices"
	"time"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
)

type ActionKind string

const (
	ActionNone     ActionKind = ""
	ActionAttach   ActionKind = "attach"
	ActionResume   ActionKind = "resume"
	ActionDemolish ActionKind = "demolish"
	ActionStop     ActionKind = "stop"
	// ActionPack packs a session up into the depot, and ActionUnpack
	// brings it back to its old lot (item 86).
	ActionPack   ActionKind = "pack"
	ActionUnpack ActionKind = "unpack"
	// ActionRestore reopens in the background the sessions open when you
	// left (item 86).
	ActionRestore ActionKind = "restore"
	// ActionCloseUp records what is open, and pauses what you chose to
	// (item 86).
	ActionCloseUp  ActionKind = "closeup"
	ActionNew      ActionKind = "new session here"
	ActionReveal   ActionKind = "reveal folder"
	ActionCopyPath ActionKind = "copy path"
	ActionHide     ActionKind = "hide project"
	ActionStar     ActionKind = "star"
	ActionUnstar   ActionKind = "unstar"
	// ActionDistrictType changes the district's type to the next of the
	// four (item 109).
	ActionDistrictType ActionKind = "district type"
	// Details folds the card's full table out under its face; generate
	// asks Claude Code for a summary, and is the only one that spends.
	ActionDetails     ActionKind = "details"
	ActionDetailsHide ActionKind = "hide details"
	// ActionMoveToGroup asks which group to move the selection to, and
	// ActionMove moves it, to Group or to none (item 87).
	ActionMoveToGroup ActionKind = "move to group"
	ActionMove        ActionKind = "move"
	ActionGenerate    ActionKind = "generate summary"
	// Read opens the reading pane, the level below the card's sentence.
	// Item 83.
	ActionRead ActionKind = "read"
	// The finder's: a line of text copied. Item 82.
	ActionCopy ActionKind = "copy"
	// ActionReviewGroups confirms the first run's groups as reviewed.
	ActionReviewGroups ActionKind = "confirm groups"
	// A lens's first actions (item 88).
	ActionFocusLine ActionKind = "focus the line"
	// ActionCopyStandup copies the selection's group's stand-up line
	// (item 90).
	ActionCopyStandup ActionKind = "copy stand-up"
	// ActionMuteWord asks which of the words that joined the selection
	// to its group to mute, and ActionMute mutes Text (item 90).
	ActionMuteWord  ActionKind = "mute a word"
	ActionMute      ActionKind = "mute"
	ActionBreakdown ActionKind = "breakdown"
	ActionOpenPR    ActionKind = "open pull request"
	ActionShowRoad  ActionKind = "show the road"
	ActionFork      ActionKind = "fork a fresh session"
	// ActionNewSession starts a session from New session, in Group or a
	// new group named GroupName, or in none (item 87).
	ActionNewSession ActionKind = "start a session"
)

const DemolishArmFor = 3 * time.Second

// Action is something the actor does through the claude CLI; Dir is the
// project directory for the actions that need one.
type Action struct {
	Kind      ActionKind
	SessionID string
	Dir       string
	// Text is what ActionCopy copies.
	Text string
	// Group is where ActionMove puts the session, or where
	// ActionNewSession starts it; empty is no group. GroupName is a new
	// group to start it in, and Prompt the prompt it starts with.
	Group     string
	GroupName string
	Prompt    string
	// Review is the first run's groups as confirmed.
	Review *group.Review
	// Open and Pinned are ActionCloseUp's open sessions and the ones
	// pinned to start with next time, and Pause the ones you chose to
	// pause; every other session is left running (item 86).
	Open, Pinned, Pause []string
}

// Light is how the scene decides night: from the sessions, or forced
// either way.
type Light int

const (
	LightAuto Light = iota
	LightNight
	LightDay
)

func (l Light) String() string {
	switch l {
	case LightNight:
		return "night"
	case LightDay:
		return "day"
	}
	return "auto"
}

// Night runs from NightFrom to NightUntil on the local clock.
const (
	NightFrom  = 21
	NightUntil = 6
)

type Scene struct {
	snapshot state.Snapshot
	// ledger is the statistics store, nil when statistics are off.
	ledger Ledger
	// focus is the group the map is focused on, if any (item 87).
	focus string
	// dive is the dive's stepped row plus one, 0 before any step, and
	// diveSide and diveRoom the room the dive's list reserves while a
	// group is focused (item 90).
	dive     int
	diveSide Side
	diveRoom float64
	ladder   state.Thresholds
	// linesHidden is whether the setting hides the groups' lines.
	linesHidden bool
	// labelsHidden is whether the setting, or its key, hides every label
	// on the map (item 102).
	labelsHidden bool
	// planOpen draws the city as its plan (item 106).
	planOpen bool
	// planShown is the cross-fade between the map and the plan, and
	// planFadeFor how long it takes.
	planShown, planFadeFor float64
	// planFrom is the camera as it was when the plan opened.
	planFrom  Camera
	planLaid  planLaid
	planCoast planCoast
	// planCovers is the chrome lying over the plan (item 112).
	planCovers []Rect
	// lastWheel is when the wheel last turned, for PlanDebounce.
	lastWheel time.Time
	light     Light
	now       func() time.Time
	location  *time.Location
	scrub     time.Duration
	topChrome float64
	bottom    float64
	left      float64
	corner    Rect
	layout    *Layout
	camera    *Camera
	city      *City
	hover     Hit
	selected  *Building
	// detailsFor is the session whose card has its details open, so a
	// new selection always starts closed; generated is what Generate
	// brought back, by session.
	detailsFor string
	// roadFor is the session whose roads are shown, and lensExpanded
	// whether the lens's panel was brought back in full (item 88).
	roadFor      string
	lensExpanded bool
	// sightings are the new kinds waiting for their first caption, the
	// first of them captioned since sightingAt.
	// triageAt is the session triage is on (item 88).
	triageAt   string
	sightings  []Kind
	sightingAt time.Time
	generated  map[string]Generated
	// veil is the names the city wears while it is shared, nil when it
	// wears its own, and real the snapshot as it came (item 168).
	veil    *Veil
	real    state.Snapshot
	width   float64
	height  float64
	touched bool
	armed   string
	armedAt time.Time
	// zoomTarget is where the wheel is taking the zoom; Animate eases the
	// camera there about zoomAnchor, or Instant snaps it.
	zoomTarget float64
	zoomAnchor Point
	instant    bool
	// merged is each session's merged-PR count at the last snapshot;
	// celebrating is when a session's count rose, for a one-shot show.
	merged      map[string]int
	celebrating map[string]time.Time
	// opened is whether a snapshot has come from outside the scene. The
	// passes that only take note of the first snapshot read it rather
	// than whether their own map is still nil: settings applied at
	// start-up re-apply the scene's empty snapshot, and that filled
	// every such map before the first real one arrived (item 169).
	opened bool
	// openOnRead is whether only a snapshot whose history is read opens
	// the city, as a recording's does, and offered whether any snapshot
	// has come (item 202).
	openOnRead, offered bool
	// known is the last snapshot's live sessions by id and voyages the
	// tugs on the river for those that came or went since.
	known   map[string]state.Session
	voyages []*Voyage
	// couriers are the refresh boats: one per usage fetch that actually
	// went out. Not sessions, so they never reach the plan. Item 56.
	couriers []*Courier
	// gaugeBoats are the usage boats and where each is driving, keyed by
	// title and drawn in gaugeOrder. gaugesManual keeps them away until
	// called, and gaugeLeaveAt is when called boats sail off. Item 79.
	gaugeBoats   map[string]*gaugeBoat
	gaugeOrder   []string
	gaugesManual bool
	gaugeLeaveAt time.Time
	// calls is each session's count of tool calls at the last snapshot,
	// lastRead when that snapshot came, loads the loads its new calls
	// sent up its hoist, working whether it was mid-turn, and struck when
	// each scaffold began coming down (item 89).
	calls    map[string]int
	lastRead time.Time
	loads    map[string][]load
	working  map[string]bool
	struck   map[string]time.Time
	// overgrowth is the road cells growing over since their connection
	// ceased. Item 85.
	overgrowth map[Cell]overgrowing
	// rungs is each session's rung at the last snapshot, and mowing the
	// lots being mowed since their session woke (item 86).
	rungs  map[string]state.Rung
	mowing map[string]time.Time
	// awake is whether each building was live at the last snapshot, and
	// settling the buildings resting or waking since.
	awake    map[string]bool
	settling map[string]settling
	// spots is where each session stood at the last snapshot, and hauls
	// the trucks carrying crates between the lots and the depot.
	spots map[string]spot
	hauls []Haul
	// keepDepot holds an empty depot open while its truck leaves.
	keepDepot bool
	// drones are the subagents' drones by session (item 89).
	drones map[string][]*flight
	// prs is each pull request's state at the last snapshot, by session
	// and URL; hoists, parked, deliveries, ribbons and celebrated are
	// what their changes set going (item 89).
	prs        map[string]string
	hoists     map[string]hoist
	parked     map[string]parking
	deliveries []Delivery
	ribbons    []ribbon
	celebrated map[string]bool
	// mail is each road's message counts at the last snapshot, and
	// messages the vans their rises sent (item 89).
	mail     map[string]mailCount
	messages []Message
	// drag is the district being dragged to another slot. Item 85.
	drag *districtDrag
	// reduced is the reduced_motion setting: everything that moves on
	// its own stands still. It is not the same as instant, which a
	// still frame sets so the camera does not have to ease into place.
	reduced bool
	// plantLights is how far the power plant's windows are on, and
	// plantFaded whether it has been faded yet, before which it follows
	// the sessions at once (item 185).
	plantLights float64
	plantFaded  bool
	// compacted, skips and jobs are each session's compactions and skip
	// as last seen, and the compaction jobs under way (item 109).
	compacted map[string]int
	skips     map[string]int
	jobs      map[string]compactionJob
	// view is the info view the city is drawn in, and generation counts
	// the snapshots taken, so the renderer can tell one city from the next.
	view       View
	generation int
	// ground is what the standing layer last drew from, and
	// groundGeneration counts the times it moved (item 206).
	ground           ground
	groundGeneration int
	// detail is how much scenery is drawn; see detail.go.
	detail Detail
	// viewTop is the largest value each ramped view found in the city,
	// and viewCats the categories each categorical one sorts it into.
	// Both are worked out once a snapshot and thrown away with it.
	viewTop     map[View]float64
	viewCats    map[View][]string
	log         *events.Log
	filter      Filter
	budget      float64
	spriteHover bool
	// memo keeps what laying out the last snapshot made, so the next lays
	// out only what changed (item 197).
	memo *memo
}

// CelebrateFor is how long a merge is shown off.
const CelebrateFor = 2500 * time.Millisecond

func NewScene(layout *Layout) *Scene {
	s := &Scene{layout: layout, ladder: state.DefaultThresholds, camera: NewCamera(), city: &City{}, now: time.Now, location: time.Local, log: events.NewLog(), memo: &memo{}}
	if layout != nil {
		// An unknown name opens on Attention rather than refusing to
		// start: the view is a preference, not data anyone can lose.
		s.view, _ = ViewByName(layout.View)
		s.seeLens(s.Lens())
	}
	return s
}

// SetClock replaces the wall clock and its zone, for tests and for a
// scrub that should not follow the machine.
func (s *Scene) SetClock(now func() time.Time, loc *time.Location) {
	s.now, s.location = now, loc
	s.applyNight()
}

// Now is the time now, in the city's time zone, unshifted by the
// scrub: what a snooze is measured from (item 88).
func (s *Scene) Now() time.Time { return s.now().In(s.location) }

// Clock is the time the light follows: now, shifted by the scrub.
func (s *Scene) Clock() time.Time {
	return s.now().In(s.location).Add(s.scrub)
}

// Scrub shifts the clock and lets the light follow it again; a zero
// shift goes back to live.
func (s *Scene) Scrub(by time.Duration) time.Duration {
	if by == 0 {
		s.scrub = 0
	} else {
		s.scrub += by
	}
	s.light = LightAuto
	s.applyNight()
	return s.scrub
}

// IsNight says whether a moment falls in the night hours.
func IsNight(t time.Time) bool {
	h := t.Hour()
	return h >= NightFrom || h < NightUntil
}

func (s *Scene) City() *City {
	s.face()
	return s.city
}

// face places the city's streets for the camera's heading if it has
// turned since they were placed, and grows over any road cell the turn
// leaves behind. Item 108.
func (s *Scene) face() {
	if s.city.Heading() == s.camera.Heading {
		return
	}
	was := s.city.StreetCells
	s.city.Face(s.camera.Heading)
	s.noteOvergrowth(was, s.now())
	s.noteGround()
}

// ToggleNight forces night on or off regardless of what is running, to
// see the city lit; the next snapshot keeps the override.
// CycleLight steps the light auto → night → day → auto and says where
// it landed.
// Light is how night is decided: from the sessions, or forced.
func (s *Scene) Light() Light {
	return s.light
}

func (s *Scene) CycleLight() Light {
	s.light = (s.light + 1) % 3
	s.applyNight()
	return s.light
}

// ToggleNight is the old n key: on the first press the city is night.
func (s *Scene) ToggleNight() bool {
	return s.CycleLight() == LightNight
}

// applyNight decides the city's light: the clock unless it is forced.
// An unattended session no longer makes it night; a lit lamp at night
// is what says a session is awake.
func (s *Scene) applyNight() {
	if s.city == nil {
		return
	}
	switch s.light {
	case LightNight:
		s.city.Night = true
	case LightDay:
		s.city.Night = false
	default:
		s.city.Night = IsNight(s.Clock())
	}
}

func (s *Scene) Camera() *Camera {
	return s.camera
}

func (s *Scene) Layout() *Layout {
	return s.layout
}

func (s *Scene) Hover() Hit {
	return s.hover
}

func (s *Scene) Selected() *Building {
	return s.selected
}

// Size is the screen the scene is laid out for.
func (s *Scene) Size() Point {
	return Point{X: s.width, Y: s.height}
}

func (s *Scene) Resize(width, height float64) {
	if width == s.width && height == s.height {
		return
	}
	s.width, s.height = width, height
	s.fit()
}

// Reframe lays the scene out for a picture of another size than the
// window, such as a 1080p GIF (item 188): an untouched view refits to
// it, and a view the user framed keeps the point at its centre and
// shows the same height of the city, so the shot is the one they framed.
func (s *Scene) Reframe(width, height float64) {
	if !s.touched || s.height <= 0 || s.camera.Zoom <= 0 {
		s.Resize(width, height)
		return
	}
	if width == s.width && height == s.height {
		return
	}
	centre := s.camera.ScreenToWorld(Point{X: s.width / 2, Y: s.height / 2})
	anchor := s.camera.ScreenToWorld(s.zoomAnchor)
	k := height / s.height
	s.width, s.height = width, height
	s.camera.Zoom *= k
	if s.zoomTarget > 0 {
		s.zoomTarget *= k
	}
	s.camera.Offset = Point{X: width / 2, Y: height / 2}.Scale(1 / s.camera.Zoom).Sub(s.camera.Project(centre))
	s.zoomAnchor = s.camera.WorldToScreen(anchor)
}

func (s *Scene) Fit() {
	s.zoomTarget = 0
	s.touched = false
	s.fit()
}

// LabelZoom is the zoom below which fixed-size labels would overlap, so the
// renderer hides them and the fit stops reserving room for them.
const LabelZoom = 0.3

func (s *Scene) fit() {
	if s.touched || len(s.city.Districts) == 0 {
		return
	}
	s.camera.FitWithInsets(s.city.Extent(), s.width, s.height, s.Insets())
	if s.camera.Zoom < LabelZoom {
		// Labels are hidden this far out, so their room goes back to the
		// map; the chrome's does not.
		s.camera.FitWithInsets(s.city.Extent(), s.width, s.height, s.chromeInsets())
	}
	// Never past the floor: a city too big to read there is shown at the
	// floor, centred on it (item 89).
	s.floored()
}

// LabelsVisible reports whether fixed-size map labels are worth drawing.
func (s *Scene) LabelsVisible() bool {
	return !s.labelsHidden && s.camera.Zoom >= LabelZoom
}

// SetLabels shows or hides every label on the map: badges, line and
// street names, and plates. Signs on posts and paint on buildings are
// part of the map and stay (item 102).
func (s *Scene) SetLabels(on bool) {
	s.labelsHidden = !on
}

// Labels is whether the map's labels are switched on, however far out
// the camera is.
func (s *Scene) Labels() bool {
	return !s.labelsHidden
}

// LandmarkLabelZoom is the zoom from which the plaza's landmarks carry
// their plates; further out the plates would land on the district
// plates around them, and the plant, hall and library are known by
// their shapes (and named on hover).
const LandmarkLabelZoom = 0.7

// LandmarkLabelsVisible reports whether the plaza landmarks' plates are
// drawn without a hover.
func (s *Scene) LandmarkLabelsVisible() bool {
	return !s.labelsHidden && s.camera.Zoom >= LandmarkLabelZoom
}

const (
	// DetailZoom is where sprites take over from the map view's flat blocks.
	DetailZoom = 0.75
	// IsoDetailZoom is the same for the isometric view, whose kit sprites
	// hold up much further out, so the fit view already shows them; below
	// it the map view's flat blocks take over. Its tiles are
	// 132 px wide and so read at half the zoom.
	IsoDetailZoom = 0.2
	// TitleZoom is where each building gets its title written under it.
	TitleZoom = 1.5
	// DistrictLabelMinWidth is the narrowest a district may be on screen
	// and still carry its name on the ground.
	DistrictLabelMinWidth = 56.0
)

// SetProjection switches how the city is drawn and refits the camera.
func (s *Scene) SetProjection(p Projection) {
	s.camera.Projection = p
	s.touched = false
	s.fit()
}

// Projection is how the city is drawn.
func (s *Scene) Projection() Projection {
	return s.camera.Projection
}

// Detailed reports whether the map is close enough for sprites to read.
func (s *Scene) Detailed() bool {
	if s.camera.Projection == Isometric {
		return s.camera.Zoom >= IsoDetailZoom
	}
	return s.camera.Zoom >= DetailZoom
}

// TitlesVisible reports whether building titles fit under the buildings.
func (s *Scene) TitlesVisible() bool {
	return s.camera.Zoom >= TitleZoom
}

// DistrictLabelVisible reports whether the district's name plate shows.
//
// Bug 40: a project's name is permanent, and the only gate left is
// whether there is room to read it. A session's title is the one that
// went hover-only — a repo name is short, there are few of them, and it
// is the label you navigate by rather than the one you read. It used to
// wait for the district to be busy, hovered or holding the selection,
// which meant the labels you needed in order to find your way were the
// ones that vanished when nothing was happening.
func (s *Scene) DistrictLabelVisible(d *District) bool {
	return !s.labelsHidden && d.Rect.Width()*s.camera.Zoom >= DistrictLabelMinWidth
}

// DistrictLabelAt is where the district's name goes on screen: on the
// floor inside the kerb when the padding has room for a line of text,
// otherwise just above the kerb. Isometric districts carry it centred
// above their top corner.
func (s *Scene) DistrictLabelAt(d *District, lineHeight, width float64) Point {
	top := s.camera.WorldToScreen(d.Rect.Min)
	if s.camera.Projection == Isometric {
		// Under the block's near vertex, which Aria asked for as "the
		// bottom" and which is also the emptiest part of a block in this
		// projection — buildings sit back from it and nothing else is
		// drawn there. Which of the four world corners is nearest
		// changes with the heading, so it is found rather than named:
		// the corner that lands lowest on screen.
		low := top
		for _, p := range [3]Point{{X: d.Rect.Max.X, Y: d.Rect.Min.Y}, d.Rect.Max, {X: d.Rect.Min.X, Y: d.Rect.Max.Y}} {
			if q := s.camera.WorldToScreen(p); q.Y > low.Y {
				low = q
			}
		}
		return low.Add(Point{X: -width / 2, Y: 4})
	}
	if DistrictPadding*s.camera.Zoom >= lineHeight+4 {
		return top.Add(Point{X: 6, Y: 4})
	}
	return top.Add(Point{X: 0, Y: -lineHeight - 2})
}

// Minimap projects the whole city into a screen box, with the camera's
// viewport marked. It projects through the camera, heading and all, so
// it turns with the city (item 170).
type Minimap struct {
	Box    Rect
	Scale  float64
	origin Point
	bounds Rect
	camera Camera
	View   Rect
}

func (s *Scene) Minimap(box Rect) Minimap {
	bounds := s.camera.Bounds(s.city.Extent())
	if bounds.Width() <= 0 || bounds.Height() <= 0 {
		return Minimap{Box: box}
	}
	scale := math.Min(box.Width()/bounds.Width(), box.Height()/bounds.Height())
	m := Minimap{Box: box, Scale: scale, bounds: bounds, camera: *s.camera}
	m.origin = Point{
		X: box.Min.X + (box.Width()-bounds.Width()*scale)/2,
		Y: box.Min.Y + (box.Height()-bounds.Height()*scale)/2,
	}
	m.View = Rect{Min: m.Project(s.camera.ScreenToWorld(Point{})), Max: m.Project(s.camera.ScreenToWorld(Point{X: s.width, Y: s.height}))}
	return m
}

// Project maps a world point into the minimap box.
func (m Minimap) Project(p Point) Point {
	return m.origin.Add(m.camera.Project(p).Sub(m.bounds.Min).Scale(m.Scale))
}

// ProjectRect maps a world rect's projected box into the minimap.
func (m Minimap) ProjectRect(r Rect) Rect {
	b := m.camera.Bounds(r)
	return Rect{
		Min: m.origin.Add(b.Min.Sub(m.bounds.Min).Scale(m.Scale)),
		Max: m.origin.Add(b.Max.Sub(m.bounds.Min).Scale(m.Scale)),
	}
}

// Insets reserve screen space for text that does not scale with the map.
// SetTopChrome reserves screen room for chrome drawn along the top, such
// as the resource strip, so a fit keeps the city below it.
func (s *Scene) SetTopChrome(px float64) {
	s.reserve(&s.topChrome, px, Point{Y: 1})
}

// TopChrome is the room reserved along the top.
func (s *Scene) TopChrome() float64 { return s.topChrome }

// SetBottomChrome reserves screen room for chrome along the bottom, such
// as the footer; the fit margin still applies when it is shorter.
func (s *Scene) SetBottomChrome(px float64) {
	s.reserve(&s.bottom, px, Point{})
}

// SetLeftChrome reserves screen room for chrome down the left, such as
// the sidebar.
func (s *Scene) SetLeftChrome(px float64) {
	s.reserve(&s.left, px, Point{X: 1})
}

// reserve changes one chrome inset and keeps the map out from under it:
// an untouched view refits to the room that is left, a view the user has
// panned or zoomed slides along the given axis by the change.
func (s *Scene) reserve(inset *float64, px float64, axis Point) {
	delta := px - *inset
	if delta == 0 {
		return
	}
	*inset = px
	if !s.touched {
		s.fit()
		return
	}
	if s.camera.Zoom > 0 {
		s.camera.Pan(axis.Scale(delta / s.camera.Zoom))
	}
}

// chromeInsets is the room the window's own chrome takes: the strip,
// the footer, the sidebar and the stream's corner, whatever the zoom.
func (s *Scene) chromeInsets() Insets {
	return s.cornerInsets(Insets{Bottom: math.Max(FitFooter, s.bottom), Top: s.topChrome, Left: s.left})
}

func (s *Scene) Insets() Insets {
	in := s.insets()
	if s.focus != "" {
		if s.diveSide == SideRight {
			in.Right += s.diveRoom
		} else {
			in.Left += s.diveRoom
		}
	}
	return in
}

func (s *Scene) insets() Insets {
	in := s.chromeInsets()
	in.Top += LabelHeight
	if s.camera.Projection == Isometric {
		// Isometric labels sit above their landmarks, inside the diamond's
		// empty corners, so nothing is reserved at the sides.
		// The chrome's top keeps the stream's corner when it is reserved
		// along the top edge (item 112).
		in.Top = math.Max(in.Top-LabelHeight, s.topChrome) + 2*LabelHeight
		return in
	}
	if len(s.city.Towers) > 0 {
		in.Left += s.city.TowerLabelWidth()
	}
	if s.city.Library.Rect.Area() > 0 {
		in.Right += LibraryLabelWidth
	}
	return in
}

// Ladder is the thresholds a stopped session climbs by (item 86).
func (s *Scene) Ladder() state.Thresholds { return s.ladder }

// SetLadder changes them, and rebuilds the city so every building takes
// its new rung at once.
func (s *Scene) SetLadder(t state.Thresholds) {
	if t == s.ladder {
		return
	}
	s.ladder = t
	s.restate()
}

// SetLedger has the city read its usage from a statistics store; nil
// is statistics turned off (item 116).
func (s *Scene) SetLedger(l Ledger) {
	s.ledger = l
	s.restate()
}

// Opened is whether a snapshot from outside has opened the city.
func (s *Scene) Opened() bool { return s.opened }

// SetSnapshot shows a snapshot from outside the scene, and the first
// one opens the city, or the first whose history is read when the scene
// opens on a read.
func (s *Scene) SetSnapshot(snapshot state.Snapshot) {
	s.real = snapshot
	s.restate()
	s.offered = true
	s.opened = s.opened || !s.openOnRead || snapshot.HeftsRead
}

// SetOpenOnRead has only a snapshot whose history is read open the city.
// A recording waits so, because until the history is read every
// building stands on the starter scale, and the snapshot that finishes
// the read raises them all at once (item 202).
func (s *Scene) SetOpenOnRead(on bool) { s.openOnRead = on }

// WaitingForRead is whether the scene holds a snapshot it will not open
// on, its history not yet read.
func (s *Scene) WaitingForRead() bool { return s.openOnRead && s.offered && !s.opened }

// OpenAnyway opens the city on the snapshot it holds, read or not, for a
// recording that has waited as long as it will; with no snapshot yet
// there is nothing to open on.
func (s *Scene) OpenAnyway() { s.opened = s.opened || s.offered }

// restate rebuilds the city from the snapshot it already holds, through
// the veil when there is one, for a change made inside the scene; it
// never opens the city.
func (s *Scene) restate() {
	snapshot := s.real
	if s.veil != nil {
		snapshot = s.veil.Snapshot(snapshot)
	}
	s.apply(snapshot)
}

func (s *Scene) apply(snapshot state.Snapshot) {
	s.snapshot = snapshot
	s.generation++
	s.viewTop, s.viewCats = nil, nil
	was := s.city.StreetCells
	s.city = build(snapshot, s.layout, s.ladder, false, s.ledger, s.memo)
	if s.veil != nil {
		s.veil.name(s.city)
	}
	s.city.Face(s.camera.Heading)
	s.noteOvergrowth(was, s.now())
	s.noteMowing(s.now())
	s.noteSettling(s.now())
	s.noteHauls(s.now())
	s.finishBuilds(s.now())
	s.noteDrones(s.now())
	s.notePRs(s.now())
	s.noteMessages(s.now())
	s.holdDepot()
	s.city.Plant.BudgetUSD = s.budget
	s.noteMerges()
	s.noteVoyages(snapshot, s.now())
	s.noteLoads(snapshot, s.now())
	s.noteCompactions(snapshot, s.now())
	s.applyNight()
	s.selected = s.reselect()
	s.hover = Hit{}
	s.fit()
	s.noteGround()
}

func (s *Scene) reselect() *Building {
	if s.selected == nil {
		return nil
	}
	for _, b := range s.city.Buildings() {
		if b.Session.ID == s.selected.Session.ID {
			return b
		}
	}
	return nil
}

// LineHoverPixels is how close, on screen, the pointer must be to a road,
// beam or power line to hover it.
const LineHoverPixels = 6.0

func (s *Scene) PointerMove(screen Point) {
	if s.planOpen {
		// The map beneath the plan is not under the pointer (item 106).
		s.hover, s.spriteHover = Hit{}, false
		return
	}
	world := s.camera.ScreenToWorld(screen)
	s.spriteHover = false
	s.hover = s.city.At(world)
	if s.hover.ground() {
		if near := s.city.Near(world, LineHoverPixels/s.camera.Zoom); near != (Hit{}) {
			s.hover = near
		}
	}
}

// PointerAway is a pointer that is over nothing at all.
func (s *Scene) PointerAway() {
	s.hover, s.spriteHover = Hit{}, false
}

// SetHover replaces the hover with what the renderer found under the
// pointer in screen space — a sprite standing taller than its footprint.
func (s *Scene) SetHover(hit Hit) {
	s.hover = hit
	s.spriteHover = true
}

// ground is a hit on something a line may run over: nothing, a park, the
// plaza or the river.
// Ground is ground() for the renderer.
func (h Hit) Ground() bool {
	return h.ground()
}

// Mover reports whether this hit is one of the five moving things. They
// are drawn on and above the buildings they belong to, so the pointer
// resolves to the building first in world space; a mover has to be
// allowed to win or it can never be pointed at at all.
func (h Hit) Mover() bool {
	return h.Subagent != nil || h.Smoke != nil || h.Car != nil || h.Flag != nil || h.Gauge != nil ||
		h.Inspector != nil || h.Delivery != nil || h.Message != nil
}

func (h Hit) ground() bool {
	if h.Building != nil || h.District != nil || h.Tower != nil {
		return false
	}
	switch h.Landmark {
	case LandmarkNone, LandmarkPlaza, LandmarkWater:
		return true
	}
	return false
}

func (s *Scene) Pan(delta Point) {
	s.touched = true
	s.camera.Pan(delta.Scale(1 / s.camera.Zoom))
	s.clamp()
}

// Wheel steps the zoom target up or down the ladder about the cursor;
// the camera eases there over the next ticks, or jumps when motion is
// reduced.
func (s *Scene) Wheel(cursor Point, amount float64) {
	if amount == 0 {
		return
	}
	s.touched = true
	target := s.camera.Zoom
	if s.zoomTarget > 0 {
		target = s.zoomTarget
	}
	// One notch out from Skyline opens the City Plan, and one in leaves
	// it (item 106).
	plans := s.camera.Projection == Isometric
	at := s.now()
	rested := s.lastWheel.IsZero() || at.Sub(s.lastWheel) >= PlanDebounce
	s.lastWheel = at
	if plans && s.planOpen {
		if amount > 0 {
			s.closePlanAt(cursor)
		}
		return
	}
	if plans && amount < 0 && rested && target <= s.SkylineZoom()*(1+1e-9) {
		s.openPlan()
		return
	}
	steps := int(math.Abs(amount) + 0.5)
	for i := 0; i < steps; i++ {
		if amount > 0 {
			target = stepAbove(target)
		} else {
			target = stepBelow(target)
		}
	}
	// Zooming out stops at the floor (item 89), and on the isometric map
	// at Skyline, the step before the plan (item 106).
	floor := FloorZoom(s.camera.Projection)
	if plans {
		floor = s.SkylineZoom()
	}
	target = math.Max(target, math.Min(floor, s.camera.Zoom))
	s.zoomTarget, s.zoomAnchor = target, cursor
	if s.instant {
		s.Animate(1)
	}
}

// SetInstant makes every zoom jump instead of ease: for reduced motion,
// and for a still frame, which cannot wait for an ease.
func (s *Scene) SetInstant(instant bool) {
	s.instant = instant
}

// SetReducedMotion stops the things that move on their own. A still
// frame does not set it, so a screenshot catches the city as it is.
func (s *Scene) SetReducedMotion(reduced bool) {
	s.reduced = reduced
}

// Animate moves the camera toward its zoom target, dt seconds on: most
// of the way each tick, and all the way once it is close. It also lets
// the light follow the clock.
func (s *Scene) Animate(dt float64) {
	s.applyNight()
	s.releaseDepot()
	s.fadePlan(dt)
	s.fadePlant(dt)
	if s.zoomTarget <= 0 || s.zoomTarget == s.camera.Zoom {
		return
	}
	zoom := s.camera.Zoom + (s.zoomTarget-s.camera.Zoom)*math.Min(1, dt*zoomEase)
	if math.Abs(s.zoomTarget-zoom) < 0.002*s.zoomTarget {
		zoom = s.zoomTarget
	}
	s.camera.SetZoomAt(s.zoomAnchor, zoom)
	s.clamp()
}

// Easing reports whether the camera is still on its way to where an
// input sent it. The traffic, the pulses and the boats never settle, so
// this is the motion a reader waits through, not all of it. Item 91.
func (s *Scene) Easing() bool {
	return s.zoomTarget > 0 && s.zoomTarget != s.camera.Zoom
}

// zoomEase is the fraction of the remaining zoom covered per second.
const zoomEase = 12.0

// clamp keeps the map under the middle of the window: the camera can
// reach the edge but never leave the plan for the void beyond it.
func (s *Scene) clamp() {
	// The view may wander off the plan as far as the island's coast and
	// the shallows past it, so the sea round the city can be seen. Item 85.
	plane := s.camera.Bounds(s.city.Island().Reach())
	if s.city.bounds.Area() == 0 {
		plane = s.camera.Bounds(s.city.Extent())
	}
	if plane.Area() == 0 {
		return
	}
	under := Point{X: s.width / 2, Y: s.height / 2}.Scale(1 / s.camera.Zoom).Sub(s.camera.Offset)
	held := Point{X: math.Min(math.Max(under.X, plane.Min.X), plane.Max.X), Y: math.Min(math.Max(under.Y, plane.Min.Y), plane.Max.Y)}
	s.camera.Offset = s.camera.Offset.Add(under.Sub(held))
}

// JumpTo selects the next building in the given state after the current
// selection, in district order, and centres the camera on it at the
// current zoom (or the detail zoom, if the map is further out than that,
// so the building can be seen). It returns nil when no building is in
// that state.
func (s *Scene) JumpTo(st state.State) *Building {
	var candidates []*Building
	for _, b := range s.city.Buildings() {
		if b.Session.State == st && !s.setAside(b) {
			candidates = append(candidates, b)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	next := candidates[0]
	if s.selected != nil {
		for i, b := range candidates {
			if b == s.selected {
				next = candidates[(i+1)%len(candidates)]
				break
			}
		}
	}
	s.selected = next
	s.CenterOn(next.Rect.Center())
	return next
}

// CenterOn pans the camera so a world point sits in the middle of the
// view, between the chrome, zooming in to the detail zoom if needed.
func (s *Scene) CenterOn(world Point) {
	s.touched = true
	if !s.Detailed() {
		if s.camera.Projection == Isometric {
			s.camera.Zoom = IsoDetailZoom
		} else {
			s.camera.Zoom = DetailZoom
		}
	}
	in := s.Insets()
	target := Point{X: in.Left + (s.width-in.Left-in.Right)/2, Y: in.Top + (s.height-in.Top-in.Bottom)/2}
	s.camera.Offset = target.Scale(1 / s.camera.Zoom).Sub(s.camera.Project(world))
}

func (s *Scene) Click(screen Point) Action {
	if s.planOpen {
		if d := s.badgeAt(screen); d != nil {
			s.flyInto(d)
		}
		return Action{}
	}
	hit := s.city.At(s.camera.ScreenToWorld(screen))
	if hit.ground() && s.spriteHover && s.hover.Building != nil {
		// The pointer is on a building's upper storeys, found by the
		// renderer's sprite pass rather than the footprint.
		hit = s.hover
	}
	s.hover = hit
	s.selected = hit.Building
	// A click selects and raises the card; it does not attach. Attaching
	// opens a terminal, which is not something to do to someone who
	// clicked a building to find out what it was. Attach is on the card,
	// and Enter is still the shortcut.
	return Action{}
}

// SetTyping is how districts with no type yet are given one; districts
// that have one keep it (item 109).
func (s *Scene) SetTyping(t Typing) {
	s.layout.SetTyping(t)
	s.restate()
}

// Actions is what the selection's card offers, in order: the way in
// first, then the session, then the project.
func (s *Scene) Actions() []ActionKind {
	if s.selected == nil {
		return nil
	}
	root := ProjectRoot(s.selected.Session.CWD)
	var kinds []ActionKind
	if s.selected.Packed {
		kinds = append(kinds, ActionUnpack, ActionResume)
		if s.detailsOpen() {
			return append(kinds, ActionDetailsHide)
		}
		return append(kinds, ActionDetails)
	}
	if s.selected.Session.State == state.Resting {
		kinds = append(kinds, ActionResume)
	} else {
		kinds = append(kinds, ActionAttach, ActionStop)
	}
	kinds = append(kinds, s.summaryActions()...)
	kinds = append(kinds, ActionPack, ActionNew, ActionReveal, ActionCopyPath, ActionHide)
	if s.layout.IsStarred(root) {
		kinds = append(kinds, ActionUnstar)
	} else {
		kinds = append(kinds, ActionStar)
	}
	kinds = append(kinds, ActionDistrictType)
	if len(s.snapshot.Groups) > 0 && s.selected.Session.State != state.Resting {
		kinds = append(kinds, ActionMoveToGroup)
	}
	if s.selected.Group != nil {
		kinds = append(kinds, ActionCopyStandup)
	}
	if s.selected.Group != nil && len(s.selected.Session.GroupWords) > 0 {
		kinds = append(kinds, ActionMuteWord)
	}
	if s.detailsOpen() {
		kinds = append(kinds, ActionDetailsHide)
	} else {
		kinds = append(kinds, ActionDetails)
	}
	return s.lensFirst(kinds)
}

// lensFirst moves the lens's first action to the front of the card, or
// adds it there when the selection has something for it to act on.
func (s *Scene) lensFirst(kinds []ActionKind) []ActionKind {
	first := s.Lens().first()
	if i := slices.Index(kinds, first); i >= 0 {
		return append([]ActionKind{first}, slices.Delete(kinds, i, i+1)...)
	}
	b := s.selected
	switch first {
	case ActionFocusLine:
		if b.Group == nil {
			return kinds
		}
	case ActionOpenPR:
		if prURL(b.Session) == "" {
			return kinds
		}
	case ActionShowRoad:
		if len(s.roadsOf(b)) == 0 {
			return kinds
		}
	case ActionFork:
		if i := slices.Index(kinds, ActionNew); i >= 0 {
			kinds = append([]ActionKind{ActionNew}, slices.Delete(kinds, i, i+1)...)
		}
	case ActionBreakdown:
	default:
		return kinds
	}
	return append([]ActionKind{first}, kinds...)
}

// Act performs one of the selection's actions: the map-only ones (hide,
// star) here, the rest as an Action for the actor. The note says what
// happened.
func (s *Scene) Act(kind ActionKind) (Action, string) {
	if s.selected == nil {
		return Action{}, "select a building first"
	}
	b := s.selected
	root := ProjectRoot(b.Session.CWD)
	name := b.Where
	switch kind {
	case ActionAttach, ActionResume:
		return s.Activate(), ""
	case ActionStop:
		return Action{Kind: ActionStop, SessionID: b.Session.ID}, "stopping " + b.Card(s.city.Time).Title
	case ActionNew:
		return Action{Kind: ActionNew, Dir: b.Session.CWD}, "new session in " + b.Session.CWD
	case ActionReveal:
		return Action{Kind: ActionReveal, Dir: b.Session.CWD}, "revealed " + b.Session.CWD
	case ActionCopyPath:
		return Action{Kind: ActionCopyPath, Dir: b.Session.CWD}, "copied " + b.Session.CWD
	case ActionHide:
		s.layout.SetHidden(root, true)
		s.selected = nil
		s.restate()
		return Action{}, "hid " + name + " (unhide it from the sidebar)"
	case ActionDetails:
		s.detailsFor = b.Session.ID
		return Action{}, ""
	case ActionDetailsHide:
		s.detailsFor = ""
		return Action{}, ""
	case ActionGenerate:
		return Action{Kind: ActionGenerate, SessionID: b.Session.ID}, "summarising " + b.Card(s.city.Time).Title
	case ActionMoveToGroup:
		return Action{Kind: ActionMoveToGroup, SessionID: b.Session.ID}, ""
	case ActionRead:
		return Action{Kind: ActionRead, SessionID: b.Session.ID}, ""
	case ActionFocusLine:
		if b.Group != nil && s.FocusGroup(b.Group.ID) {
			return Action{}, "focused " + b.Group.Name
		}
		return Action{}, "not in a group"
	case ActionMuteWord:
		return Action{Kind: ActionMuteWord, SessionID: b.Session.ID}, ""
	case ActionCopyStandup:
		if b.Group == nil {
			return Action{}, "not in a group"
		}
		line := s.Standup(b.Group.ID)
		return Action{Kind: ActionCopy, Text: line}, "copied stand-up: " + line
	case ActionBreakdown:
		return Action{Kind: ActionBreakdown}, ""
	case ActionFork:
		return Action{Kind: ActionFork, SessionID: b.Session.ID, Dir: b.Session.CWD}, "forking " + b.Card(s.city.Time).Title
	case ActionOpenPR:
		if url := prURL(b.Session); url != "" {
			return Action{Kind: ActionOpenPR, Text: url}, "opening " + url
		}
		return Action{}, "no pull request"
	case ActionShowRoad:
		s.roadFor = b.Session.ID
		return Action{}, fmt.Sprintf("showing %s", plural(len(s.roadsOf(b)), "road"))
	case ActionStar:
		s.layout.SetStarred(root, true)
		return Action{}, "starred " + name
	case ActionUnstar:
		s.layout.SetStarred(root, false)
		return Action{}, "unstarred " + name
	case ActionDistrictType:
		next := NextDistrictType(s.layout.DistrictType(root))
		s.layout.SetDistrictType(root, next)
		s.restate()
		return Action{}, name + " is now " + string(next)
	}
	return Action{}, ""
}

// SetBudget is the daily spend to measure the strip and the plant
// against; it lands on the next snapshot and this one.
func (s *Scene) SetBudget(usd float64) {
	s.budget = usd
	if s.city != nil {
		s.city.Plant.BudgetUSD = usd
	}
}

// AddEvents takes what the daemon (or the feed, without one) logged
// since the last batch; the map keeps no diff of its own.
func (s *Scene) AddEvents(fresh []events.Event) {
	s.log.Add(fresh...)
}

// Events is everything that has happened, newest first.
func (s *Scene) Events() []events.Event {
	return s.log.Events()
}

// Away is what needed you or went wrong after a moment, newest first:
// the list shown when the window comes back into focus.
func (s *Scene) Away(since time.Time) []events.Event {
	return s.log.Since(since, events.NeedsYou, events.Error)
}

// noteMerges starts a celebration for every session whose merged-PR
// count rose since the last snapshot; the first snapshot only takes
// note, so an old merge does not fire on start-up.
func (s *Scene) noteMerges() {
	first := !s.opened
	if s.merged == nil {
		s.merged = map[string]int{}
		s.celebrating = map[string]time.Time{}
	}
	now := s.now()
	for _, b := range s.city.Buildings() {
		if !first && b.Merged > s.merged[b.Session.ID] {
			s.celebrating[b.Session.ID] = now
		}
		s.merged[b.Session.ID] = b.Merged
	}
}

// Celebration is how far along a session's merge show is, 0 at the
// start and 1 at the end, or -1 when it is not celebrating.
func (s *Scene) Celebration(id string) float64 {
	at, ok := s.celebrating[id]
	if !ok {
		return -1
	}
	p := float64(s.now().Sub(at)) / float64(CelebrateFor)
	if p >= 1 {
		delete(s.celebrating, id)
		return -1
	}
	return p
}

// Select picks a session by id and centres on it; false when it is not
// on the map.
// Deselect closes the selection, and with it the card that carries the
// verbs. Bug 43a: nothing cleared it, so the one surface the mouse
// opens was the one surface Escape could not close.
func (s *Scene) Deselect() {
	s.selected = nil
}

func (s *Scene) Select(id string) bool {
	for _, b := range s.city.Buildings() {
		if b.Session.ID == id {
			s.selected = b
			s.CenterOn(b.Rect.Center())
			return true
		}
	}
	return false
}

// CenterOnProject centres the camera on a project's district.
func (s *Scene) CenterOnProject(root string) bool {
	for _, d := range s.city.Districts {
		if d.Root == root {
			s.CenterOn(d.Rect.Center())
			return true
		}
	}
	return false
}

// Turn faces the camera a quarter turn on about the middle of the window.
func (s *Scene) Turn() int {
	s.touched = true
	s.camera.Turn(1, s.Size().Scale(0.5))
	s.face()
	s.clamp()
	return s.camera.Heading
}

// Unhide puts a hidden project back on the map.
func (s *Scene) Unhide(root string) {
	s.layout.SetHidden(root, false)
	s.restate()
}

// NewHere is the c key: a new session in the selected building's
// directory, else the hovered district's root.
func (s *Scene) NewHere() (Action, string) {
	dir := ""
	switch {
	case s.selected != nil:
		dir = s.selected.Session.CWD
	case s.hover.District != nil && !s.hover.District.Depot:
		dir = s.hover.District.Root
	}
	if dir == "" {
		return Action{}, "select a building or hover a district first"
	}
	return Action{Kind: ActionNew, Dir: dir}, "new session in " + dir
}

// SelectedCard is the selection's card with its actions, for pinning
// beside the building.
func (s *Scene) SelectedCard() (Card, *Building, bool) {
	if s.selected == nil {
		return Card{}, nil, false
	}
	card := s.buildingCard(s.selected)
	card.Open = s.detailsOpen()
	for _, kind := range s.Actions() {
		card.Actions = append(card.Actions, string(kind))
	}
	return card, s.selected, true
}

// Activate is what a click or Enter does to the selected building: attach
// a live session, resume a parked one, nothing when nothing is selected.
func (s *Scene) Activate() Action {
	if s.selected == nil {
		return Action{}
	}
	if s.selected.Session.State == state.Resting {
		return Action{Kind: ActionResume, SessionID: s.selected.Session.ID}
	}
	return Action{Kind: ActionAttach, SessionID: s.selected.Session.ID}
}

func (s *Scene) Demolish(now time.Time) (Action, string) {
	if s.selected == nil {
		return Action{}, "select a building first"
	}
	id := s.selected.Session.ID
	if s.armed == id && now.Sub(s.armedAt) <= DemolishArmFor {
		s.armed = ""
		return Action{Kind: ActionDemolish, SessionID: id}, "demolished " + s.selected.Card(now).Title
	}
	s.armed, s.armedAt = id, now
	return Action{}, "press d again within 3s to demolish " + s.selected.Card(now).Title
}

func (s *Scene) Card() (Card, bool) { return s.CardFor(s.hover) }

// CardFor is the card for what a hit is on, starting with what kind of
// thing it is (item 88).
func (s *Scene) CardFor(h Hit) (Card, bool) {
	card, ok := s.cardFor(h)
	if ok && card.Kind == "" {
		card.Kind = KindOf(h).Caption()
	}
	return card, ok
}

func (s *Scene) cardFor(h Hit) (Card, bool) {
	switch {
	// The movers first: a drone circles a roof and a flag stands on one,
	// so the building's hit rect covers both. Pointing at the drone has
	// to answer about the drone.
	case h.Station != nil:
		return StationCard(*h.Station.Line, h.Station.Index), true
	case h.Gauge != nil:
		return GaugeCard(h.Gauge.Gauge, h.Gauge.Age), true
	case h.Car != nil:
		return CarCard(h.Car.Road, h.Car.From, h.Car.To), true
	case h.Flag != nil:
		return FlagCard(h.Flag.PR), true
	case h.Delivery != nil:
		return DeliveryCard(*h.Delivery), true
	case h.Message != nil:
		return MessageCard(*h.Message), true
	case h.Inspector != nil:
		return InspectorCard(h.Inspector), true
	case h.Subagent != nil:
		return SubagentCard(h.Subagent), true
	case h.Smoke != nil:
		return SmokeCard(h.Smoke), true
	case h.Landmark == LandmarkPlant:
		return s.city.Plant.Card(), true
	case h.Landmark == LandmarkTower && h.Tower != nil:
		return h.Tower.Card(), true
	case h.Landmark == LandmarkLibrary:
		return s.city.Library.Card(), true
	case h.Landmark == LandmarkHall:
		return s.city.Hall.Card(s.city.Time), true
	case h.Building != nil:
		return s.buildingCard(h.Building), true
	case h.District != nil:
		return h.District.Card(), true
	case h.Road != nil:
		return h.Road.Card(), true
	case h.Beam != nil:
		return h.Beam.Card(), true
	case h.Line != nil:
		return h.Line.Card(), true
	case h.Train != nil:
		return h.Train.Card(), true
	case h.Voyage != nil:
		return h.Voyage.Card(), true
	case h.Courier != nil:
		return h.Courier.Card(), true
	case h.Landmark == LandmarkWater:
		return Card{Title: "river", Lines: []string{"runs through the island to the sea"}}, true
	case h.Landmark == LandmarkFountain:
		return fountainCard(), true
	case h.Landmark == LandmarkPlaza:
		return plazaCard(), true
	}
	return Card{}, false
}

// TogglePack is Shift+P: pack the selected session up, or unpack it when
// it is a crate in the depot.
func (s *Scene) TogglePack() Action {
	if s.selected == nil {
		return Action{}
	}
	if s.selected.Packed {
		return Action{Kind: ActionUnpack, SessionID: s.selected.Session.ID}
	}
	return Action{Kind: ActionPack, SessionID: s.selected.Session.ID}
}

// OpenSessions is what closing up would stop: every live session on the
// map, most recently active first (item 86).
func (s *Scene) OpenSessions() []state.Session {
	var open []state.Session
	for _, sess := range s.snapshot.Sessions {
		if state.Live(sess.State) && !sess.Packed {
			open = append(open, sess)
		}
	}
	slices.SortStableFunc(open, func(a, b state.Session) int { return b.LastActivity.Compare(a.LastActivity) })
	return open
}
