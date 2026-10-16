package city

import (
	"math"
	"path/filepath"
	"time"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

type ActionKind string

const (
	ActionNone     ActionKind = ""
	ActionAttach   ActionKind = "attach"
	ActionResume   ActionKind = "resume"
	ActionDemolish ActionKind = "demolish"
	ActionStop     ActionKind = "stop"
	ActionNew      ActionKind = "new session here"
	ActionReveal   ActionKind = "reveal folder"
	ActionCopyPath ActionKind = "copy path"
	ActionHide     ActionKind = "hide project"
	ActionStar     ActionKind = "star"
	ActionUnstar   ActionKind = "unstar"
	// The summary toggles a paragraph open on the card; generate asks
	// Claude Code for one, and is the only one of the three that spends.
	ActionSummary     ActionKind = "summary"
	ActionSummaryHide ActionKind = "hide summary"
	ActionGenerate    ActionKind = "generate summary"
	// The finder's: a stopped session resumed in a terminal in its own
	// directory, a new one started where a deleted conversation was, and
	// a line of text copied. Item 82.
	ActionResumeIn ActionKind = "resume in a terminal"
	ActionStartIn  ActionKind = "start in a terminal"
	ActionCopy     ActionKind = "copy"
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
	// focus is the group the map is focused on, if any (item 87).
	focus     string
	light     Light
	now       func() time.Time
	location  *time.Location
	scrub     time.Duration
	topChrome float64
	bottom    float64
	left      float64
	layout    *Layout
	camera    *Camera
	city      *City
	hover     Hit
	selected  *Building
	// summaryFor is the session whose card has its summary open, so a
	// new selection always starts closed; generated is what Generate
	// brought back, by session.
	summaryFor string
	generated  map[string]Generated
	width      float64
	height     float64
	touched    bool
	armed      string
	armedAt    time.Time
	// zoomTarget is where the wheel is taking the zoom; Animate eases the
	// camera there about zoomAnchor, or Instant snaps it.
	zoomTarget float64
	zoomAnchor Point
	instant    bool
	// merged is each session's merged-PR count at the last snapshot;
	// celebrating is when a session's count rose, for a one-shot show.
	merged      map[string]int
	celebrating map[string]time.Time
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
	// tools is each live session's tool call at the last snapshot and
	// trips the workers out on the road because one started.
	tools map[string]string
	trips map[string]*Trip
	// overgrowth is the road cells growing over since their connection
	// ceased. Item 85.
	overgrowth map[Cell]overgrowing
	// drag is the district being dragged to another slot. Item 85.
	drag *districtDrag
	// reduced is the reduced_motion setting: everything that moves on
	// its own stands still. It is not the same as instant, which a
	// still frame sets so the camera does not have to ease into place.
	reduced bool
	// view is the info view the city is drawn in, and generation counts
	// the snapshots taken, so the renderer can tell one city from the next.
	view       View
	generation int
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
}

// CelebrateFor is how long a merge is shown off.
const CelebrateFor = 2500 * time.Millisecond

func NewScene(layout *Layout) *Scene {
	s := &Scene{layout: layout, camera: NewCamera(), city: &City{}, now: time.Now, location: time.Local, log: events.NewLog()}
	if layout != nil {
		// An unknown name opens on Attention rather than refusing to
		// start: the view is a preference, not data anyone can lose.
		s.view, _ = ViewByName(layout.View)
	}
	return s
}

// SetClock replaces the wall clock and its zone, for tests and for a
// scrub that should not follow the machine.
func (s *Scene) SetClock(now func() time.Time, loc *time.Location) {
	s.now, s.location = now, loc
	s.applyNight()
}

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
	return s.city
}

// ToggleNight forces night on or off regardless of what is running, to
// see the city lit; the next snapshot keeps the override.
// CycleLight steps the light auto → night → day → auto and says where
// it landed.
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
}

// LabelsVisible reports whether fixed-size map labels are worth drawing.
func (s *Scene) LabelsVisible() bool {
	return s.camera.Zoom >= LabelZoom
}

// LandmarkLabelZoom is the zoom from which the plaza's landmarks carry
// their plates; further out the plates would land on the district
// plates around them, and the plant, hall and library are known by
// their shapes (and named on hover).
const LandmarkLabelZoom = 0.7

// LandmarkLabelsVisible reports whether the plaza landmarks' plates are
// drawn without a hover.
func (s *Scene) LandmarkLabelsVisible() bool {
	return s.camera.Zoom >= LandmarkLabelZoom
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
	return d.Rect.Width()*s.camera.Zoom >= DistrictLabelMinWidth
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
// viewport marked.
type Minimap struct {
	Box        Rect
	Scale      float64
	origin     Point
	bounds     Rect
	projection Projection
	View       Rect
}

func (s *Scene) Minimap(box Rect) Minimap {
	bounds := s.camera.Bounds(s.city.Extent())
	if bounds.Width() <= 0 || bounds.Height() <= 0 {
		return Minimap{Box: box}
	}
	scale := math.Min(box.Width()/bounds.Width(), box.Height()/bounds.Height())
	m := Minimap{Box: box, Scale: scale, bounds: bounds, projection: s.camera.Projection}
	m.origin = Point{
		X: box.Min.X + (box.Width()-bounds.Width()*scale)/2,
		Y: box.Min.Y + (box.Height()-bounds.Height()*scale)/2,
	}
	m.View = Rect{Min: m.Project(s.camera.ScreenToWorld(Point{})), Max: m.Project(s.camera.ScreenToWorld(Point{X: s.width, Y: s.height}))}
	return m
}

// Project maps a world point into the minimap box.
func (m Minimap) Project(p Point) Point {
	return m.origin.Add(m.projection.Apply(p).Sub(m.bounds.Min).Scale(m.Scale))
}

// ProjectRect maps a world rect's projected box into the minimap.
func (m Minimap) ProjectRect(r Rect) Rect {
	b := m.projection.Bounds(r)
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
// the footer and the sidebar, whatever the zoom.
func (s *Scene) chromeInsets() Insets {
	return Insets{Bottom: math.Max(FitFooter, s.bottom), Top: s.topChrome, Left: s.left}
}

func (s *Scene) Insets() Insets {
	in := s.chromeInsets()
	in.Top += LabelHeight
	if s.camera.Projection == Isometric {
		// Isometric labels sit above their landmarks, inside the diamond's
		// empty corners, so nothing is reserved at the sides.
		in.Top = 2*LabelHeight + s.topChrome
		return in
	}
	if len(s.city.Towers) > 0 {
		in.Left += s.city.TowerLabelWidth()
	}
	if s.city.Library.Rect.Area() > 0 {
		in.Right = LibraryLabelWidth
	}
	return in
}

func (s *Scene) SetSnapshot(snapshot state.Snapshot) {
	s.snapshot = snapshot
	s.generation++
	s.viewTop, s.viewCats = nil, nil
	was := s.city.StreetCells
	s.city = Build(snapshot, s.layout)
	s.noteOvergrowth(was, s.now())
	s.city.Plant.BudgetUSD = s.budget
	s.noteMerges()
	s.noteVoyages(snapshot, s.now())
	s.noteTrips(snapshot, s.now())
	s.applyNight()
	s.selected = s.reselect()
	s.hover = Hit{}
	s.fit()
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
	world := s.camera.ScreenToWorld(screen)
	s.spriteHover = false
	s.hover = s.city.At(world)
	if s.hover.ground() {
		if near := s.city.Near(world, LineHoverPixels/s.camera.Zoom); near != (Hit{}) {
			s.hover = near
		}
	}
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
	return h.Worker != nil || h.Subagent != nil || h.Smoke != nil || h.Car != nil || h.Flag != nil || h.Gauge != nil
}

func (h Hit) ground() bool {
	if h.Building != nil || h.District != nil || h.Tower != nil {
		return false
	}
	switch h.Landmark {
	case LandmarkNone, LandmarkPark, LandmarkPlaza, LandmarkWater:
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
	steps := int(math.Abs(amount) + 0.5)
	for i := 0; i < steps; i++ {
		if amount > 0 {
			target = stepAbove(target)
		} else {
			target = stepBelow(target)
		}
	}
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
	plane := s.camera.Bounds(s.city.Extent())
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
		if b.Session.State == st {
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

// Actions is what the selection's card offers, in order: the way in
// first, then the session, then the project.
func (s *Scene) Actions() []ActionKind {
	if s.selected == nil {
		return nil
	}
	root := ProjectRoot(s.selected.Session.CWD)
	var kinds []ActionKind
	if s.selected.Session.State == state.Parked {
		kinds = append(kinds, ActionResume)
	} else {
		kinds = append(kinds, ActionAttach, ActionStop)
	}
	kinds = append(kinds, s.summaryActions()...)
	kinds = append(kinds, ActionNew, ActionReveal, ActionCopyPath, ActionHide)
	if s.layout.IsStarred(root) {
		kinds = append(kinds, ActionUnstar)
	} else {
		kinds = append(kinds, ActionStar)
	}
	return kinds
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
	name := filepath.Base(root)
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
		s.SetSnapshot(s.snapshot)
		return Action{}, "hid " + name + " (unhide it from the sidebar)"
	case ActionSummary:
		s.summaryFor = b.Session.ID
		return Action{}, ""
	case ActionSummaryHide:
		s.summaryFor = ""
		return Action{}, ""
	case ActionGenerate:
		return Action{Kind: ActionGenerate, SessionID: b.Session.ID}, "summarising " + b.Card(s.city.Time).Title
	case ActionStar:
		s.layout.SetStarred(root, true)
		return Action{}, "starred " + name
	case ActionUnstar:
		s.layout.SetStarred(root, false)
		return Action{}, "unstarred " + name
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
	first := s.merged == nil
	if first {
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
	s.clamp()
	return s.camera.Heading
}

// Unhide puts a hidden project back on the map.
func (s *Scene) Unhide(root string) {
	s.layout.SetHidden(root, false)
	s.SetSnapshot(s.snapshot)
}

// NewHere is the c key: a new session in the selected building's
// directory, else the hovered district's root.
func (s *Scene) NewHere() (Action, string) {
	dir := ""
	switch {
	case s.selected != nil:
		dir = s.selected.Session.CWD
	case s.hover.District != nil && !s.hover.District.Storage:
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
	card := s.selected.Card(s.city.Time)
	if s.summaryOpen() {
		lines, para := s.summary(s.selected.Session)
		card.Lines = append(card.Lines, lines...)
		card.Para = para
	}
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
	if s.selected.Session.State == state.Parked {
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

func (s *Scene) Card() (Card, bool) {
	switch {
	// The movers first: a drone circles a roof and a flag stands on one,
	// so the building's hit rect covers both. Pointing at the drone has
	// to answer about the drone.
	case s.hover.Gauge != nil:
		return GaugeCard(s.hover.Gauge.Gauge, s.hover.Gauge.Age), true
	case s.hover.Car != nil:
		return CarCard(s.hover.Car.Road, s.hover.Car.From, s.hover.Car.To), true
	case s.hover.Flag != nil:
		return FlagCard(s.hover.Flag.PR), true
	case s.hover.Subagent != nil:
		return SubagentCard(s.hover.Subagent), true
	case s.hover.Worker != nil:
		return WorkerCard(s.hover.Worker), true
	case s.hover.Smoke != nil:
		return SmokeCard(s.hover.Smoke), true
	case s.hover.Landmark == LandmarkPlant:
		return s.city.Plant.Card(), true
	case s.hover.Landmark == LandmarkTower && s.hover.Tower != nil:
		return s.hover.Tower.Card(), true
	case s.hover.Landmark == LandmarkLibrary:
		return s.city.Library.Card(), true
	case s.hover.Landmark == LandmarkHall:
		return s.city.Hall.Card(s.city.Time), true
	case s.hover.Building != nil:
		return s.hover.Building.Card(s.city.Time), true
	case s.hover.District != nil:
		return s.hover.District.Card(), true
	case s.hover.Road != nil:
		return s.hover.Road.Card(), true
	case s.hover.Beam != nil:
		return s.hover.Beam.Card(), true
	case s.hover.Line != nil:
		return s.hover.Line.Card(), true
	case s.hover.Train != nil:
		return s.hover.Train.Card(), true
	case s.hover.Voyage != nil:
		return s.hover.Voyage.Card(), true
	case s.hover.Courier != nil:
		return s.hover.Courier.Card(), true
	case s.hover.Landmark == LandmarkWater:
		return Card{Title: "river", Lines: []string{"the map's edge on this side"}}, true
	case s.hover.Landmark == LandmarkPark && s.hover.Park != nil:
		return s.hover.Park.Card(), true
	case s.hover.Landmark == LandmarkFountain:
		return fountainCard(), true
	case s.hover.Landmark == LandmarkPlaza:
		return plazaCard(), true
	}
	return Card{}, false
}
