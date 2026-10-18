package city

import (
	"math"
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
)

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
	// At is the badge's centre on screen, and Rect its box.
	At   Point
	Rect Rect
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
	// Legend is the strip the legend draws in.
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

// openPlan turns the map into the plan.
func (s *Scene) openPlan() {
	s.planOpen = true
	s.zoomTarget = 0
	if s.instant {
		s.planShown = 1
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
	inner := Rect{
		Min: area.Min.Add(Point{X: PlanBadgeWidth/2 + planPad, Y: PlanBadgeHeight/2 + planPad}),
		Max: area.Max.Sub(Point{X: PlanBadgeWidth/2 + planPad, Y: PlanBadgeHeight/2 + planPad}),
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

	plan := CityPlan{Legend: legend, frame: frame}
	at := map[*District]Point{}
	tones := map[state.Move]bool{}
	for _, d := range districts {
		sign := d.Sign()
		b := PlanBadge{District: d, Name: sign.Name, Move: sign.Move, Tone: sign.Tone, Sessions: len(d.Buildings)}
		for _, bd := range d.Buildings {
			if bd.Session.State == state.NeedsYou {
				b.NeedsYou++
			}
			b.Subagents += bd.Session.SubagentsInFlight
		}
		b.At = frame.toScreen(d.Rect.Center())
		r := d.Rect
		b.Lot = [4]Point{
			frame.toScreen(r.Min),
			frame.toScreen(Point{X: r.Max.X, Y: r.Min.Y}),
			frame.toScreen(r.Max),
			frame.toScreen(Point{X: r.Min.X, Y: r.Max.Y}),
		}
		b.Rect = RectAt(b.At.X-PlanBadgeWidth/2, b.At.Y-PlanBadgeHeight/2, PlanBadgeWidth, PlanBadgeHeight)
		at[d] = b.At
		tones[b.Tone] = true
		plan.Badges = append(plan.Badges, b)
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
