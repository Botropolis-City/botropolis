package render

import (
	"math"
	"time"

	"botropolis.city/botropolis/pkg/city"
)

// Every road is two-way, and every road vehicle drives the lane on the
// right of its travel. Item 153.

// laneOffset is how far a lane's centre stands from the road's centre
// line: half a lane.
const laneOffset = 0.2 * city.CellSize

// lotLaneOffset is half a lane inside a block (item 222). A lot's lanes
// run as close as 6.6 units to a building's front, so a vehicle a
// road's half-lane off them stood in the building, or over the kerb
// where the lane ran along the lot's edge; a lot lane is one vehicle
// wide.
const lotLaneOffset = 0.0625 * city.CellSize

// bendRadius is the radius of a road's centre line round a bend, as the
// road kit draws its bend: a quarter ring about the cell's corner, from
// one side's middle to the other's (item 222).
const bendRadius = city.CellSize / 2

// bendSteps is how many legs a bend's quarter ring is drawn as.
const bendSteps = 8

// rightOf is laneOffset to the right of travel along d: with y south,
// heading east the right is south.
func rightOf(d city.Point) city.Point {
	return rightBy(d, laneOffset)
}

// rightBy is off to the right of travel along d.
func rightBy(d city.Point, off float64) city.Point {
	n := math.Hypot(d.X, d.Y)
	if n == 0 {
		return city.Point{}
	}
	return city.Point{X: -d.Y / n * off, Y: d.X / n * off}
}

// keepRight is the lane a vehicle drives following path on the roads:
// the path moved laneOffset to the right of travel. At a corner the two
// legs' lanes meet at their mitre, so the vehicle keeps its lane round
// it; where the path turns back, the lane holds both ends, so turning
// back is a short drive across the road.
func keepRight(path []city.Point) []city.Point {
	return lane(path, false, nil)
}

// keepRightLoop is keepRight for a path driven round and round: its end
// joins its start the way its corners join.
func keepRightLoop(path []city.Point) []city.Point {
	return lane(path, true, nil)
}

// lane is the lane a vehicle drives following path, inBlock saying which
// of its legs are inside a block: those keep a lot lane's half-lane and
// the rest a road's, and a right-angled turn between two road legs goes
// round the bend's quarter ring rather than across its kerb (item 222).
// A nil inBlock is all road.
func lane(path []city.Point, loop bool, inBlock func(city.Point) bool) []city.Point {
	var pts []city.Point
	for _, p := range path {
		if len(pts) == 0 || p != pts[len(pts)-1] {
			pts = append(pts, p)
		}
	}
	if len(pts) < 2 {
		return pts
	}
	closed := loop && len(pts) > 2 && pts[0] == pts[len(pts)-1]
	if closed {
		pts = pts[:len(pts)-1]
	}
	n := len(pts)
	offs := make([]float64, n)
	for i := range n {
		offs[i] = laneOffset
		if inBlock != nil && inBlock(pts[i].Add(pts[(i+1)%n]).Scale(0.5)) {
			offs[i] = lotLaneOffset
		}
	}
	pts, offs = roundBends(pts, offs, closed)
	n = len(pts)
	join := func(i int) []city.Point {
		a := pts[i].Sub(pts[(i-1+n)%n])
		b := pts[(i+1)%n].Sub(pts[i])
		ra, rb := rightBy(a, offs[(i-1+n)%n]), rightBy(b, offs[i])
		la, lb := math.Hypot(a.X, a.Y), math.Hypot(b.X, b.Y)
		cos := (a.X*b.X + a.Y*b.Y) / (la * lb)
		cross := a.X*b.Y - a.Y*b.X
		if cos < -0.5 || math.Abs(cross) < 1e-9*la*lb || (cos > 0.9 && ra != rb) {
			if ra == rb {
				return []city.Point{pts[i].Add(ra)}
			}
			return []city.Point{pts[i].Add(ra), pts[i].Add(rb)}
		}
		gap := rb.Sub(ra)
		t := (gap.X*b.Y - gap.Y*b.X) / cross
		return []city.Point{pts[i].Add(ra).Add(a.Scale(t))}
	}
	var out []city.Point
	if closed {
		first := join(0)
		out = append(out, first[len(first)-1])
		for i := 1; i < n; i++ {
			out = append(out, join(i)...)
		}
		return append(out, first...)
	}
	out = append(out, pts[0].Add(rightBy(pts[1].Sub(pts[0]), offs[0])))
	for i := 1; i < n-1; i++ {
		out = append(out, join(i)...)
	}
	return append(out, pts[n-1].Add(rightBy(pts[n-1].Sub(pts[n-2]), offs[n-2])))
}

// roundBends turns each right-angled corner between two road legs into
// the bend's quarter ring, from the middle of the side the path comes in
// by to the middle of the side it leaves by. offs is each leg's
// half-lane, the leg from each point to the next; a leg cut short by a
// ring keeps its own, and the ring's legs are road. A corner whose legs
// are too short to hold the ring keeps its point.
func roundBends(pts []city.Point, offs []float64, closed bool) ([]city.Point, []float64) {
	n := len(pts)
	var out []city.Point
	var outOffs []float64
	used := 0.0
	for i := range n {
		prev, next := pts[(i-1+n)%n], pts[(i+1)%n]
		a, b := pts[i].Sub(prev), next.Sub(pts[i])
		la, lb := math.Hypot(a.X, a.Y), math.Hypot(b.X, b.Y)
		square := math.Abs(a.X*b.X+a.Y*b.Y) < 1e-9*la*lb
		end := !closed && (i == 0 || i == n-1)
		road := offs[(i-1+n)%n] == laneOffset && offs[i] == laneOffset
		if end || !square || !road || la < used+bendRadius || lb < bendRadius {
			out = append(out, pts[i])
			outOffs = append(outOffs, offs[i])
			used = 0
			continue
		}
		ua, ub := a.Scale(1/la), b.Scale(1/lb)
		centre := pts[i].Sub(ua.Scale(bendRadius)).Add(ub.Scale(bendRadius))
		for k := 0; k <= bendSteps; k++ {
			th := math.Pi / 2 * float64(k) / bendSteps
			out = append(out, centre.Sub(ub.Scale(bendRadius*math.Cos(th))).Add(ua.Scale(bendRadius*math.Sin(th))))
			outOffs = append(outOffs, laneOffset)
		}
		outOffs[len(outOffs)-1] = offs[i]
		used = bendRadius
	}
	return out, outOffs
}

// thereAndBack is path driven to its end and back to its start.
func thereAndBack(path []city.Point) []city.Point {
	out := append([]city.Point{}, path...)
	for i := len(path) - 2; i >= 0; i-- {
		out = append(out, path[i])
	}
	return out
}

// inLane is where a vehicle share of the way along path stands, and
// which way it faces.
func inLane(path []city.Point, share float64, inBlock func(city.Point) bool) (city.Point, city.Point) {
	if len(path) == 0 {
		return city.Point{}, city.Point{}
	}
	l := lane(path, false, inBlock)
	return pointAlong(l, min(1, max(0, share))*pathLength(l))
}

// carPlace is where a traffic car phase of the way round its street, there
// and back, stands.
func carPlace(path []city.Point, phase float64, inBlock func(city.Point) bool) (city.Point, city.Point) {
	loop := lane(thereAndBack(path), true, inBlock)
	return pointAlong(loop, phase*pathLength(loop))
}

// messagePlace is where a message's van stands; a carriage on the rails
// keeps to the line.
func messagePlace(m city.Message, now time.Time, inBlock func(city.Point) bool) (city.Point, city.Point) {
	if m.Train {
		return m.At(now), m.At(now.Add(100 * time.Millisecond)).Sub(m.At(now))
	}
	share := m.Progress(now)
	if m.Fades {
		share = 1
	}
	return inLane(m.Path, share, inBlock)
}

// deliveryPlace is where the delivery truck stands.
func deliveryPlace(d city.Delivery, now time.Time, inBlock func(city.Point) bool) (city.Point, city.Point) {
	share := d.Progress(now)
	if d.Fades {
		share = 0
	}
	return inLane(d.Path, share, inBlock)
}

// haulPlace is where the depot's truck stands on its way.
func haulPlace(h city.Haul, now time.Time) (city.Point, city.Point) {
	dir := h.To.Sub(h.From)
	return h.At(now).Add(rightOf(dir)), dir
}

// jobPlace is where a site truck Along of its job stands: from 0 at the
// street to 1 at its stop on the way in, and on to 2 at the street on
// the way out.
func jobPlace(in, out []city.Point, along float64, inBlock func(city.Point) bool) (city.Point, city.Point) {
	trip := lane(append(append([]city.Point{}, in...), out...), false, inBlock)
	if len(trip) < 2 {
		return inLane(in, 0, inBlock)
	}
	inLen := pathLength(lane(in, false, inBlock))
	dist := min(1, max(0, along)) * inLen
	if along > 1 {
		dist = inLen + min(1, along-1)*(pathLength(trip)-inLen)
	}
	return pointAlong(trip, dist)
}

// inspectorPlace is where the inspector's car stands: driving in along +x
// along its lot's front to park at the kerb, and out by turning back
// across its lot lane.
func inspectorPlace(in city.Inspector) (city.Point, city.Point) {
	from := city.Point{X: in.Park.X - city.InspectorRun, Y: in.Park.Y}
	loop := lane(thereAndBack([]city.Point{from, in.Park}), true, func(city.Point) bool { return true })
	dist := in.Parked * city.InspectorRun
	if in.Leaving {
		dist = city.InspectorRun + (1-in.Parked)*(city.InspectorRun+2*lotLaneOffset)
	}
	return pointAlong(loop, dist)
}
