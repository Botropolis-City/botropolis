package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"os"

	"context"
	"math"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/ui"
)

// The usage gauges: boats on the river. Item 49, axes corrected by bug
// 52.
//
// The percentage is read ALONG the river — one end 0%, the other 100%,
// bow pointing at 100% — and the three boats are held apart ACROSS it,
// a lane each. Aria: "The boats should be going bottom to top and face
// that direction. They shouldn't all be in the same line. They should
// be spread out left to right as well."
//
// Item 49 had these the other way round, which is why "where is 0% and
// 100%?" had no answer on the frame: the reading ran across a river two
// cells wide, the shortest dimension available, while the long axis
// carried nothing at all. On this axis the resolution problem that
// forced the widening stops existing — a 25% step is tens of cells
// rather than tens of pixels — and the boats stop travelling, because
// their position along the river IS the reading. A gauge that drifts
// cannot be read.

// bowHome is the world direction a hull's bow points when it is drawn
// unturned.
//
// Set by eye against a frame, and it has to be. The geometry does not
// determine it: on ship-cargo-a the tallest structure and the hull's
// narrowest slice point in opposite directions, so "the bridge is aft"
// and "the hull tapers forward" disagree, and the liner tapers at both
// ends so its outline has no bow to find at all. For "which way does
// this face", a person looking at the picture is the specification.
//
// It is south, and r254 shipped it as north. I decided the cargo ship's
// yellow deckhouse was its stern by convention, then read a frame
// expecting to see that and saw it. Aria looked once: "the boats are
// faced backwards". Taking the deckhouse as the BOW instead, the four
// cut rotations agree — it sits left at rotation 0, top-left at 90,
// right at 180 and bottom-right at 270, and south is the one world
// direction that projects to all four of those at their headings.
//
// If this ever looks wrong again, check it against a frame rather than
// against the model. The model cannot settle it.
var bowHome = city.Point{X: 0, Y: 1}

// rotate turns a world direction by a quarter per 90 degrees, the same
// sense as the camera's own heading.
func rotate(p city.Point, deg int) city.Point {
	switch ((deg/90)%4 + 4) % 4 {
	case 1:
		return city.Point{X: -p.Y, Y: p.X}
	case 2:
		return city.Point{X: -p.X, Y: -p.Y}
	case 3:
		return city.Point{X: p.Y, Y: -p.X}
	}
	return p
}

// bowTurn is the rotation to draw a hull at so its bow points along a
// world direction. Drawing a piece turned by t shows it rotated by -t
// relative to the camera, so the home direction is turned back.
func bowTurn(dir city.Point) int {
	for _, turn := range []int{0, 90, 180, 270} {
		if r := rotate(bowHome, -turn); nearly(r.X, dir.X) && nearly(r.Y, dir.Y) {
			return turn
		}
	}
	return 0
}

func nearly(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// gaugeUpstream is the way along the river that reads as "more":
// whichever of the two points up the screen at this heading.
//
// It has to be chosen per heading and cannot be fixed to a bank. The
// river runs north-south, and world north projects UP the screen at
// headings 0 and 270 but DOWN at 90 and 180 — so a gauge nailed to one
// end would literally run backwards at half the headings. Same ruling
// as the monument's near corner, and for the same reason.
func gaugeUpstream(cam *city.Camera, from, to city.Point) city.Point {
	along := to.Sub(from)
	if length := math.Hypot(along.X, along.Y); length > 0 {
		along = along.Scale(1 / length)
	}
	back := city.Point{X: -along.X, Y: -along.Y}
	if cam.Project(back).Y < cam.Project(along).Y {
		return back
	}
	return along
}

// gaugeRightward is the way across the river the lanes are laid out
// along: whichever of the two points right on screen, so the boats keep
// the same left-to-right order however the camera turns.
func gaugeRightward(cam *city.Camera) city.Point {
	east := city.Point{X: 1}
	west := city.Point{X: -1}
	if cam.Project(west).X > cam.Project(east).X {
		return west
	}
	return east
}

// gaugeFacing is the turn that points a hull's bow at 100%.
func gaugeFacing(cam *city.Camera, from, to city.Point) int {
	return bowTurn(gaugeUpstream(cam, from, to))
}

// gaugeBankClear is the water kept between a hull and the bank.
//
// Bug 55: the lanes used to reserve the WIDEST hull's half beam at both
// banks, which put the widest hull itself exactly on the line — a
// clearance of zero, produced by the arithmetic rather than chosen. Half
// a map tile is the smallest amount of water that reads as water.
//
// A whole tile does not fit. Three cells of river spend 47.7 units on
// the liner's beam and need 33.8 between the big boat and the medium
// one, which leaves about 14 either side; at a full tile the binding
// pair closes to 6 units and the guard fails. That is the river's width
// talking, not this number, and a fourth cell would buy it.
var gaugeBankClear = city.Tile / 2

// gaugeEndroom keeps a boat reading 0% or 100% wholly inside the run
// rather than hanging off its end — half the longest hull.
const gaugeEndroom = 26.0

// gaugeMarkInset sets the channel markers just inside the bank, clear
// of the outermost lane, so a buoy never fouls a boat it is there to be
// read against.
const gaugeMarkInset = 6.0

// gaugeBeams is how wide each of the first n boats is, in lane order,
// so the lanes can be pinned by the hull that actually rides at each
// end rather than by the widest one everywhere.
func gaugeBeams(n int) []float64 {
	beams := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		beams = append(beams, gaugeHulls[city.SizeFor(i)].beam)
	}
	return beams
}

// gaugeLanes is where each of n boats rides across the river.
func gaugeLanes(width float64, n int) []float64 {
	return ui.GaugeLanes(width, gaugeBeams(n), gaugeBankClear)
}

// gaugeRun is the stretch a reading is drawn along: the run's midpoint,
// the direction of 100%, and how far it is from 0% to 100%.
func gaugeRun(cam *city.Camera, from, to city.Point) (mid, up city.Point, length float64) {
	up = gaugeUpstream(cam, from, to)
	mid = city.Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
	length = max(0, math.Hypot(to.X-from.X, to.Y-from.Y)-2*gaugeEndroom)
	return mid, up, length
}

// gaugeAt is where a boat holds station: along the river by its
// reading, across it by its lane.
func gaugeAt(cam *city.Camera, from, to city.Point, lane float64, g city.Gauge) city.Point {
	return gaugeTrackAt(cam, from, to, lane, ui.GaugeShare(g.Percent), 0, 0)
}

// gaugeTrackAt is where a boat is at a point on its track. 0 to 1 is
// the run, read as a share; below it the boat is back towards the way
// in, reaching back at city.GaugeOffLow, and above it on towards the
// way out, reaching ahead at city.GaugeOffHigh. Item 79.
func gaugeTrackAt(cam *city.Camera, from, to city.Point, lane, track, back, ahead float64) city.Point {
	mid, up, length := gaugeRun(cam, from, to)
	start := mid.Sub(up.Scale(length / 2))
	var along float64
	switch {
	case track < 0:
		along = track / -city.GaugeOffLow * back
	case track > 1:
		along = length + (track-1)/(city.GaugeOffHigh-1)*ahead
	default:
		along = length * track
	}
	return start.Add(up.Scale(along)).Add(gaugeRightward(cam).Scale(lane))
}

// gaugeReach is how far past each end of the run the river goes, plus
// a hull, so a boat at either end of its track is off the water and off
// the map rather than parked at the corner.
func gaugeReach(cam *city.Camera, from, to, bandFrom, bandTo city.Point) (back, ahead float64) {
	mid, up, length := gaugeRun(cam, from, to)
	dot := func(p city.Point) float64 { d := p.Sub(mid); return d.X*up.X + d.Y*up.Y }
	lo, hi := min(dot(bandFrom), dot(bandTo)), max(dot(bandFrom), dot(bandTo))
	clear := gaugeHulls[city.GaugeBig].extent
	return max(0, -lo-length/2) + clear, max(0, hi-length/2) + clear
}

// gaugeMark is one channel marker: where it floats, and whether it
// marks the limit rather than a station on the way.
type gaugeMark struct {
	At    city.Point
	Limit bool
}

// gaugeStations are the readings the markers stand for. Both ends and
// the middle: item 49 marked halfway and the limit only, and across the
// wrong axis, which is why the frame could not say where 0% was. The
// ends are the thing that was missing.
var gaugeStations = [3]float64{0, 0.5, 1}

// gaugeMarks are the references a boat is read against, set along the
// bank so they never sit in a lane.
func gaugeMarks(cam *city.Camera, from, to city.Point, width float64) []gaugeMark {
	mid, up, length := gaugeRun(cam, from, to)
	start := mid.Sub(up.Scale(length / 2))
	bank := gaugeRightward(cam).Scale(max(0, width/2-gaugeMarkInset))
	out := make([]gaugeMark, 0, len(gaugeStations))
	for _, share := range gaugeStations {
		out = append(out, gaugeMark{
			At:    start.Add(up.Scale(length * share)).Add(bank),
			Limit: share == 1,
		})
	}
	return out
}

// Each rank's hull and how long it is drawn. The hull carries the
// difference in profile and the length carries the difference in size,
// so a boat is told from its neighbour by shape before scale — which is
// what survives at the zoom where scale stops being readable.
//
// Mapped to rank rather than to a named window, because the ordering is
// generic: a plan with a different set of limits keeps working, which
// hardcoding three hull names would throw away.
type gaugeHull struct {
	piece string
	// beam is how wide the hull draws across the river, in world units,
	// measured off the cut sprite at z1 and converted at 2*IsoScale.
	// Stated here and re-derived from the art by
	// TestRiverFitsThreeHullsAbreast, so the lanes and the hulls cannot
	// drift apart the way a written-down number and its source do.
	beam float64
	// extent is how long the boat's longest side is drawn, in the
	// atlas's own pixels. Stated as a size rather than a scale, because
	// a scale means different things to sprites of different sizes —
	// the liner and the cargo ship drew the same length when each had
	// its own shrink factor chosen from the models.
	extent float64
}

// The three ranks step down by a bit under two thirds each, measured on
// the longest side so a tall hull and a long one are compared fairly.
var gaugeHulls = map[city.GaugeSize]gaugeHull{
	city.GaugeBig:    {piece: kitGaugeLiner, beam: 47.7, extent: 142},
	city.GaugeMedium: {piece: kitGaugeCargo, beam: 19.8, extent: 93},
	city.GaugeSmall:  {piece: kitGaugeSail, beam: 17.2, extent: 60},
}

// courierExtent draws the refresh boat between the sail and the cargo
// ship: big enough to notice crossing the map, small enough that it is
// plainly not one of the three that carry a reading.
const courierExtent = 74.0

// unit is a direction normalised to one of the four world axes, for
// picking a hull's rotation from a path.
func unit(d city.Point) city.Point {
	if math.Abs(d.X) > math.Abs(d.Y) {
		return city.Point{X: math.Copysign(1, d.X)}
	}
	return city.Point{Y: math.Copysign(1, d.Y)}
}

// buoyExtent keeps a marker smaller than the boats it measures: a
// reference should be read past, not looked at.
const buoyExtent = 26.0

// riverRun is the stretch of river the gauges are read along, and how
// wide the water is for the lanes to spread across.
//
// Not the whole river. Bug 55: its ends are the map's corners, so a low
// reading — which is most readings — drew every boat where nobody
// looks. City.GaugeRun puts the scale beside the city instead, and the
// clearance it is given is half the longest hull, so a boat at 0% or
// 100% is wholly inside the run rather than hanging off its end.
func riverRun(c *city.City) (from, to city.Point, width float64, ok bool) {
	return c.GaugeRun(gaugeEndroom)
}

// drawGauge floats one boat at its reading, bow pointing at 100%.
func (g *Game) drawGauge(screen *ebiten.Image, cam *city.Camera, at city.Point, size city.GaugeSize, turn int) city.Rect {
	hull, ok := gaugeHulls[size]
	if !ok {
		hull = gaugeHulls[city.GaugeSmall]
	}
	return g.kitSized(screen, cam, hull.piece, turn, at, hull.extent, nil)
}

// usageAge is how old the reading the boats carry is.
func (g *Game) usageAge() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.usage.Age(timeNow())
}

// takeUsage hands the game loop a reading that landed since it last
// looked, and whether a probe came back — the scene is the loop's, so
// the probe's goroutine leaves both here rather than touching it.
func (g *Game) takeUsage() (gauges []city.Gauge, fresh, landed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	fresh, landed = g.usageFresh, g.usageLanded
	g.usageFresh, g.usageLanded = false, false
	if fresh && g.usage.Shape() != claude.ShapeUnknown {
		gauges = city.Gauges(g.usage)
	}
	return gauges, fresh, landed
}

// followUsageInScene brings the boats up to date with the last reading
// and, by hand, calls them up when a probe the user asked for comes
// back. Item 79.
func (g *Game) followUsageInScene() {
	gauges, fresh, landed := g.takeUsage()
	if fresh {
		g.scene.SetGauges(gauges)
	}
	if landed && g.usageEvery <= 0 {
		g.scene.VisitGauges()
	}
	if g.usageDue() {
		g.refreshUsage()
	}
}

// usageDue is whether the schedule wants a probe: refreshing is on an
// interval, and neither the reading nor the last probe is younger than
// it. Never while filming or taking a still, which read a fixture home
// and must not reach the network. And never while nobody can see the
// window (item 160): a probe is a whole Claude Code, and boats nobody
// is looking at do not need it. The u key asks whenever it is pressed.
func (g *Game) usageDue() bool {
	if g.usageEvery <= 0 || g.record != "" || g.screenshot != "" {
		return false
	}
	if !watching(g.driven) {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	last := g.usage.FetchedAt
	if g.usageAskedAt.After(last) {
		last = g.usageAskedAt
	}
	return timeNow().Sub(last) >= g.usageEvery
}

// readUsage re-reads Claude Code's cache off disk. It is a file read,
// so it costs nothing and needs no schedule — the boats follow whatever
// Claude Code last fetched.
func (g *Game) readUsage() {
	u, ok := claude.ReadUtilization(g.home)
	if !ok {
		return
	}
	g.mu.Lock()
	g.usage, g.usageFresh = u, true
	g.mu.Unlock()
}

// followUsage re-reads the cache when the file has changed since it was
// last read, so the boats follow Claude Code's own refreshes without the
// city being restarted. It runs as each snapshot lands: a stat, and a
// read only when there is something new.
func (g *Game) followUsage() {
	info, err := os.Stat(claude.UtilizationPath(g.home))
	if err != nil || info.ModTime().Equal(g.usageRead) {
		return
	}
	g.usageRead = info.ModTime()
	g.readUsage()
}

// refreshUsage makes Claude Code refetch the figures and then re-reads
// them, off the frame.
//
// This is the only thing the probe is still for. Reading the cache is
// free, so the boats follow it without any schedule; the probe exists
// because nothing else makes Claude Code go and ask. It writes a
// transcript, which is why it runs in a directory of its own. It spends
// no tokens but it is a network fetch, so it runs when the u key asks,
// or on usage_refresh's interval when one is set (item 79).
func (g *Game) refreshUsage() bool {
	if !g.mayAskUsage() {
		return false
	}
	probe := g.probe
	if probe == nil {
		probe = func(ctx context.Context) { claude.RefreshUtilization(ctx, "") }
	}
	go func() {
		defer func() {
			g.mu.Lock()
			g.usageInFlight, g.usageLanded = false, true
			g.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), usageTimeout)
		defer cancel()
		probe(ctx)
		// Whether or not the probe succeeded, re-read: Claude Code may
		// have refetched for its own reasons in the meantime.
		g.readUsage()
	}()
	return true
}

// mayAskUsage decides whether to set a probe going, and claims the slot
// if so. Separated from the work because the work is a subprocess: the
// decision is what has rules worth testing, and a test that reached the
// CLI would write the very transcripts the fence exists to keep out.
func (g *Game) mayAskUsage() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.usageInFlight || timeNow().Sub(g.usageAskedAt) < usageDebounce {
		return false
	}
	g.usageInFlight, g.usageAskedAt = true, timeNow()
	return true
}

// usageDebounce is how long a reading stands before the key will fetch
// another. Bug 54: Aria, on a screen recording of five tugs at once —
// "we should add a debounce so we don't get a ton of them."
//
// A refresh is user-initiated, so the only floor without one is how
// fast the key can be pressed, and every press is a session that
// arrives by river and re-plans the map. The figures are percentages of
// a five-hour and a seven-day window; nothing in them can move enough
// in half a minute to be worth a boat.
const usageDebounce = 30 * time.Second

// usageTimeout is generous: /usage took 25–45 s on 2026-10-21 (item 160),
// once it began reading the transcripts for its insights, and a slow
// machine is not a failure.
const usageTimeout = 90 * time.Second
