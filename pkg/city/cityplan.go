package city

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// The City Plan. Item 106.
//
// One wheel notch out past Skyline, the city is drawn as a flat plan:
// each district a badge at its own place, the groups' lines between
// them, and the usage readings in a legend. Zooming in returns to the
// isometric map at Skyline, with the district under the pointer still
// under it.
//
// The plan faces the way the map does: a world point is turned by the
// camera's heading and laid out as (x-y, x+y), the isometric map without
// its 2:1 squash, so what is left of something on the map is left of it
// on the plan.

const (
	// PlanBadgeWidth and PlanBadgeHeight are a badge's box on screen.
	PlanBadgeWidth  = 168.0
	PlanBadgeHeight = 62.0
	// PlanLegendHeight is the strip at the foot of the plan kept for the
	// legend.
	PlanLegendHeight = 64.0
	// planPad keeps badges off the edges of the plan.
	planPad = 24.0
	// planMaxScale is the most screen pixels a world unit is given, so a
	// city of two districts is not flung to opposite corners.
	planMaxScale = 0.9
	// PlanCompactHeight is a compact badge's height, and its width runs
	// with its name up to planCompactMaxWidth.
	PlanCompactHeight   = 22.0
	planCompactMaxWidth = 150.0
	planCompactMinWidth = 56.0
	// PlanCompactChar is roughly one character's width on a compact
	// badge, for sizing it before any text is measured.
	PlanCompactChar = 8.0
	// planFullCover is the share of the plan full badges may cover
	// before every badge goes compact.
	planFullCover = 0.3
	// planGap is the least space between two badges.
	planGap = 4.0
	// planReach is how far, in pixels, a badge may be moved from its
	// place to find room, and planStep how finely the room is searched.
	planReach = 480.0
	planStep  = 3.0
)

// size is a badge's box: full, or compact to fit its name, its dot and
// how many need you.
func (b PlanBadge) size() (float64, float64) {
	if !b.Compact {
		return PlanBadgeWidth, PlanBadgeHeight
	}
	chars := len([]rune(b.Name))
	if b.NeedsYou > 0 {
		chars += len(strconv.Itoa(b.NeedsYou)) + 1
	}
	w := 36 + PlanCompactChar*float64(chars)
	return math.Max(planCompactMinWidth, math.Min(planCompactMaxWidth, w)), PlanCompactHeight
}

// LayoutPlanBadges places boxes so each is at least gap from every
// other, inside the area: in order, each keeps its own spot if it is
// free, and otherwise takes the nearest free spot. So two boxes on the
// same spot part on the axis on which they overlap least, and boxes
// already apart do not move.
func LayoutPlanBadges(in []Rect, area Rect, gap float64) []Rect {
	order := make([]int, len(in))
	for i := range order {
		order[i] = i
	}
	return layoutPlan(in, area, gap, order)
}

// planOffsets are the moves a badge may make to find room, nearest
// first; ties go up before down and left before right, so the result
// never depends on the sort.
var planOffsets = func() []Point {
	var out []Point
	steps := int(planReach / planStep)
	for iy := -steps; iy <= steps; iy++ {
		for ix := -steps; ix <= steps; ix++ {
			p := Point{X: float64(ix) * planStep, Y: float64(iy) * planStep}
			if math.Hypot(p.X, p.Y) <= planReach {
				out = append(out, p)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := math.Hypot(out[i].X, out[i].Y), math.Hypot(out[j].X, out[j].Y)
		if di != dj {
			return di < dj
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}()

// planLaid is the last layout, kept because the plan is laid out every
// frame and its badges move only when the city or the window does.
type planLaid struct {
	in    []Rect
	area  Rect
	order []int
	out   []Rect
}

// planLayout is layoutPlan, remembered.
func (s *Scene) planLayout(in []Rect, area Rect, order []int) []Rect {
	c := s.planLaid
	if c.area == area && slices.Equal(c.in, in) && slices.Equal(c.order, order) {
		return c.out
	}
	out := layoutPlan(in, area, planGap, order)
	s.planLaid = planLaid{in: slices.Clone(in), area: area, order: slices.Clone(order), out: out}
	return out
}

// layoutPlan is LayoutPlanBadges placing the boxes in the given order,
// so the earlier ones keep their spots.
func layoutPlan(in []Rect, area Rect, gap float64, order []int) []Rect {
	out := make([]Rect, len(in))
	placed := make([]Rect, 0, len(in))
	free := func(r Rect) bool {
		if r.Min.X < area.Min.X-1e-9 || r.Min.Y < area.Min.Y-1e-9 || r.Max.X > area.Max.X+1e-9 || r.Max.Y > area.Max.Y+1e-9 {
			return false
		}
		grown := r.Inset(-gap)
		for _, p := range placed {
			if grown.Overlaps(p) {
				return false
			}
		}
		return true
	}
	for _, i := range order {
		want := keepInside(in[i], area)
		got := want
		for _, by := range planOffsets {
			if r := shift(want, by); free(r) {
				got = r
				break
			}
		}
		out[i] = got
		placed = append(placed, got)
	}
	return out
}

func shift(r Rect, by Point) Rect { return Rect{Min: r.Min.Add(by), Max: r.Max.Add(by)} }

// keepInside slides a box back inside an area it would leave.
func keepInside(r Rect, area Rect) Rect {
	var by Point
	switch {
	case r.Min.X < area.Min.X:
		by.X = area.Min.X - r.Min.X
	case r.Max.X > area.Max.X:
		by.X = area.Max.X - r.Max.X
	}
	switch {
	case r.Min.Y < area.Min.Y:
		by.Y = area.Min.Y - r.Min.Y
	case r.Max.Y > area.Max.Y:
		by.Y = area.Max.Y - r.Max.Y
	}
	return shift(r, by)
}

// PlanBadge is a district on the plan.
type PlanBadge struct {
	District *District
	Name     string
	Move     string
	Tone     state.Move
	Sessions int
	NeedsYou int
	// Subagents is how many subagents are in flight across the district.
	Subagents int
	// Place is where the district lies on the plan; At is the badge's
	// centre once badges are pushed apart, and Rect its box.
	Place Point
	At    Point
	Rect  Rect
	// Compact is a badge shrunk to its name and a status dot.
	Compact bool
	// Lot is the district's ground on the plan, corner by corner.
	Lot [4]Point
}

// PlanLine is a group's line on the plan, through its stations' badges
// in station order.
type PlanLine struct {
	Group  *state.Group
	Points []Point
	Moves  []state.Move
}

// PlanReading is one usage limit, as the plan's legend reads it.
type PlanReading struct {
	Title    string
	Percent  float64
	Severity string
}

// CityPlan is everything the plan draws, laid out for the window.
type CityPlan struct {
	Badges   []PlanBadge
	Lines    []PlanLine
	Readings []PlanReading
	// Tones are the moves the badges carry, most urgent first, for the
	// legend to key.
	Tones []state.Move
	// Area is where badges stand, and Legend the strip the legend draws
	// in.
	Area   Rect
	Legend Rect
	frame  planFrame
}

// planFrame takes the plan's own plane to the screen and back.
type planFrame struct {
	heading     int
	scale       float64
	centre      Point
	planeCentre Point
}

func (f planFrame) plane(world Point) Point {
	c := Camera{Heading: f.heading}
	t := c.turn(world)
	return Point{X: t.X - t.Y, Y: t.X + t.Y}
}

func (f planFrame) toScreen(world Point) Point {
	return f.centre.Add(f.plane(world).Sub(f.planeCentre).Scale(f.scale))
}

func (f planFrame) toWorld(screen Point) Point {
	if f.scale <= 0 {
		return Point{}
	}
	m := screen.Sub(f.centre).Scale(1 / f.scale).Add(f.planeCentre)
	c := Camera{Heading: f.heading}
	return c.unturn(Point{X: (m.X + m.Y) / 2, Y: (m.Y - m.X) / 2})
}

// ToWorld is the world point under a screen point on the plan.
func (p CityPlan) ToWorld(screen Point) Point { return p.frame.toWorld(screen) }

// ToScreen is where a world point lies on the plan.
func (p CityPlan) ToScreen(world Point) Point { return p.frame.toScreen(world) }

// PlanDebounce is how long the wheel must rest before a notch out from
// Skyline opens the plan, so a quick zoom-out stops at Skyline and a
// slow one carries on into the plan.
const PlanDebounce = 350 * time.Millisecond

// PlanFade is how long, in seconds, the map and the plan take to
// cross-fade into each other at a liveliness level.
func PlanFade(l Liveliness) float64 {
	switch l {
	case Still:
		return 0.12
	case Calm:
		return 0.5
	case Lively:
		return 0.25
	}
	return 0.35
}

// CityPlanOpen reports whether the city is drawn as its plan.
func (s *Scene) CityPlanOpen() bool { return s.planOpen }

// PlanShown is how far the cross-fade to the plan has come: 0 is the
// isometric map, 1 the plan.
func (s *Scene) PlanShown() float64 { return s.planShown }

// SetPlanFade sets how long the cross-fade between the map and the plan
// takes, in seconds.
func (s *Scene) SetPlanFade(seconds float64) { s.planFadeFor = seconds }

// fadePlan moves the cross-fade dt seconds on.
func (s *Scene) fadePlan(dt float64) {
	target := 0.0
	if s.planOpen {
		target = 1
	}
	fadeFor := s.planFadeFor
	if fadeFor <= 0 {
		fadeFor = PlanFade(Steady)
	}
	if s.instant {
		s.planShown = target
		return
	}
	step := dt / fadeFor
	if s.planShown < target {
		s.planShown = math.Min(target, s.planShown+step)
	} else {
		s.planShown = math.Max(target, s.planShown-step)
	}
}

// planFading reports whether the cross-fade is under way.
func (s *Scene) planFading() bool {
	return (s.planOpen && s.planShown < 1) || (!s.planOpen && s.planShown > 0)
}

// SkylineZoom is the furthest the isometric map zooms out: the whole
// city, or the floor when the city does not read whole there.
func (s *Scene) SkylineZoom() float64 {
	return math.Max(s.FitZoom(), FloorZoom(s.camera.Projection))
}

// openPlan turns the map into the plan, remembering where the camera
// was so the key can put it back.
func (s *Scene) openPlan() {
	if s.zoomTarget > 0 {
		s.camera.SetZoomAt(s.zoomAnchor, s.zoomTarget)
		s.clamp()
	}
	s.planOpen = true
	s.planFrom = *s.camera
	s.zoomTarget = 0
	if s.instant {
		s.planShown = 1
	}
}

// ToggleCityPlan opens the plan from wherever the map is, or closes it
// back to where the camera was when it opened.
func (s *Scene) ToggleCityPlan() {
	if s.camera.Projection != Isometric {
		return
	}
	if s.planOpen {
		s.CloseCityPlan()
		return
	}
	s.openPlan()
}

// CloseCityPlan returns to the map as it was when the plan opened.
func (s *Scene) CloseCityPlan() {
	if !s.planOpen {
		return
	}
	s.planOpen = false
	if s.instant {
		s.planShown = 0
	}
	from := s.planFrom
	from.Heading = s.camera.Heading
	*s.camera = from
	s.zoomTarget = 0
	s.touched = true
}

// badgeAt is the district whose badge is under a screen point on the
// plan.
func (s *Scene) badgeAt(screen Point) *District {
	plan := s.CityPlan()
	for i := len(plan.Badges) - 1; i >= 0; i-- {
		if plan.Badges[i].Rect.Contains(screen) {
			return plan.Badges[i].District
		}
	}
	return nil
}

// flyInto leaves the plan for the map at Skyline, centred on a district,
// and eases in until the district fills the view.
func (s *Scene) flyInto(d *District) {
	s.planOpen = false
	if s.instant {
		s.planShown = 0
	}
	s.touched = true
	in := s.Insets()
	middle := Point{X: in.Left + (s.width-in.Left-in.Right)/2, Y: in.Top + (s.height-in.Top-in.Bottom)/2}
	s.camera.Zoom = s.SkylineZoom()
	s.camera.Offset = middle.Scale(1 / s.camera.Zoom).Sub(s.camera.Project(d.Rect.Center()))
	fit := *s.camera
	fit.FitWithInsets(d.Rect, s.width, s.height, in)
	s.zoomTarget = math.Min(MaxZoom, math.Max(fit.Zoom, s.camera.Zoom))
	s.zoomAnchor = middle
	if s.instant {
		s.Animate(1)
	}
}

// closePlanAt returns to the isometric map at Skyline's zoom, keeping
// the world point under the pointer on the plan under it on the map.
func (s *Scene) closePlanAt(cursor Point) {
	world := s.CityPlan().ToWorld(cursor)
	s.planOpen = false
	if s.instant {
		s.planShown = 0
	}
	s.touched = true
	s.zoomTarget = 0
	s.camera.Zoom = s.SkylineZoom()
	s.camera.Offset = cursor.Scale(1 / s.camera.Zoom).Sub(s.camera.Project(world))
	s.clamp()
}

// planArea is where badges may stand: the window inside the chrome, less
// the legend's strip.
func (s *Scene) planArea() (Rect, Rect) {
	in := s.Insets()
	area := Rect{Min: Point{X: in.Left, Y: in.Top}, Max: Point{X: s.width - in.Right, Y: s.height - in.Bottom}}
	legend := Rect{Min: Point{X: area.Min.X, Y: area.Max.Y - PlanLegendHeight}, Max: area.Max}
	area.Max.Y -= PlanLegendHeight
	return area, legend
}

// CityPlan lays the plan out for the window as it is now.
func (s *Scene) CityPlan() CityPlan {
	area, legend := s.planArea()
	var districts []*District
	for _, d := range s.city.Districts {
		if !d.Depot {
			districts = append(districts, d)
		}
	}
	frame := planFrame{heading: s.camera.Heading, centre: area.Center()}
	compact := float64(len(districts))*PlanBadgeWidth*PlanBadgeHeight > planFullCover*area.Area()
	halfW, halfH := PlanBadgeWidth/2, PlanBadgeHeight/2
	if compact {
		halfW, halfH = planCompactMaxWidth/2, PlanCompactHeight/2
	}
	inner := Rect{
		Min: area.Min.Add(Point{X: halfW + planPad, Y: halfH + planPad}),
		Max: area.Max.Sub(Point{X: halfW + planPad, Y: halfH + planPad}),
	}
	if len(districts) > 0 {
		first := frame.plane(districts[0].Rect.Center())
		bounds := Rect{Min: first, Max: first}
		for _, d := range districts[1:] {
			p := frame.plane(d.Rect.Center())
			bounds.Min.X, bounds.Min.Y = math.Min(bounds.Min.X, p.X), math.Min(bounds.Min.Y, p.Y)
			bounds.Max.X, bounds.Max.Y = math.Max(bounds.Max.X, p.X), math.Max(bounds.Max.Y, p.Y)
		}
		frame.planeCentre = bounds.Center()
		frame.scale = planMaxScale
		if w := bounds.Width(); w > 0 {
			frame.scale = math.Min(frame.scale, math.Max(0, inner.Width())/w)
		}
		if h := bounds.Height(); h > 0 {
			frame.scale = math.Min(frame.scale, math.Max(0, inner.Height())/h)
		}
		frame.centre = inner.Center()
	}

	plan := CityPlan{Area: area, Legend: legend, frame: frame}
	tones := map[state.Move]bool{}
	rects := make([]Rect, 0, len(districts))
	for _, d := range districts {
		sign := d.Sign()
		b := PlanBadge{District: d, Name: sign.Name, Move: sign.Move, Tone: sign.Tone, Sessions: len(d.Buildings), Compact: compact}
		for _, bd := range d.Buildings {
			if bd.Session.State == state.NeedsYou {
				b.NeedsYou++
			}
			b.Subagents += bd.Session.SubagentsInFlight
		}
		b.Place = frame.toScreen(d.Rect.Center())
		r := d.Rect
		b.Lot = [4]Point{
			frame.toScreen(r.Min),
			frame.toScreen(Point{X: r.Max.X, Y: r.Min.Y}),
			frame.toScreen(r.Max),
			frame.toScreen(Point{X: r.Min.X, Y: r.Max.Y}),
		}
		w, h := b.size()
		rects = append(rects, RectAt(b.Place.X-w/2, b.Place.Y-h/2, w, h))
		tones[b.Tone] = true
		plan.Badges = append(plan.Badges, b)
	}
	order := make([]int, len(plan.Badges))
	for i := range order {
		order[i] = i
	}
	// A district needing you keeps its own spot; the rest make room.
	sort.SliceStable(order, func(i, j int) bool {
		return plan.Badges[order[i]].NeedsYou > 0 && plan.Badges[order[j]].NeedsYou == 0
	})
	at := map[*District]Point{}
	for i, r := range s.planLayout(rects, area, order) {
		plan.Badges[i].Rect = r
		plan.Badges[i].At = r.Center()
		at[plan.Badges[i].District] = r.Center()
	}
	for _, m := range []state.Move{state.MoveYours, state.MoveTheirs, state.MoveAgents, state.MoveNobody} {
		if tones[m] {
			plan.Tones = append(plan.Tones, m)
		}
	}
	for _, l := range s.Lines() {
		pl := PlanLine{Group: l.Group}
		for _, st := range l.Stations {
			p, ok := at[st.District]
			if !ok {
				continue
			}
			pl.Points = append(pl.Points, p)
			pl.Moves = append(pl.Moves, st.Move)
		}
		if len(pl.Points) > 1 {
			plan.Lines = append(plan.Lines, pl)
		}
	}
	for _, title := range s.gaugeOrder {
		if b, ok := s.gaugeBoats[title]; ok {
			plan.Readings = append(plan.Readings, PlanReading{Title: b.gauge.Title, Percent: b.gauge.Percent, Severity: b.gauge.Severity})
		}
	}
	return plan
}
