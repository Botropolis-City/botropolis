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

// rightOf is laneOffset to the right of travel along d: with y south,
// heading east the right is south.
func rightOf(d city.Point) city.Point {
	n := math.Hypot(d.X, d.Y)
	if n == 0 {
		return city.Point{}
	}
	return city.Point{X: -d.Y / n * laneOffset, Y: d.X / n * laneOffset}
}

// keepRight is the lane a vehicle drives following path: the path moved
// laneOffset to the right of travel. At a corner the two legs' lanes
// meet at their mitre, so the vehicle keeps its lane round it; where
// the path turns back, the lane holds both ends, so turning back is a
// short drive across the road.
func keepRight(path []city.Point) []city.Point {
	return lane(path, false)
}

// keepRightLoop is keepRight for a path driven round and round: its end
// joins its start the way its corners join.
func keepRightLoop(path []city.Point) []city.Point {
	return lane(path, true)
}

func lane(path []city.Point, loop bool) []city.Point {
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
	join := func(i int) []city.Point {
		a := pts[i].Sub(pts[(i-1+n)%n])
		b := pts[(i+1)%n].Sub(pts[i])
		ra, rb := rightOf(a), rightOf(b)
		cos := (a.X*b.X + a.Y*b.Y) / (math.Hypot(a.X, a.Y) * math.Hypot(b.X, b.Y))
		if cos < -0.5 {
			return []city.Point{pts[i].Add(ra), pts[i].Add(rb)}
		}
		return []city.Point{pts[i].Add(ra.Add(rb).Scale(1 / (1 + cos)))}
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
	out = append(out, pts[0].Add(rightOf(pts[1].Sub(pts[0]))))
	for i := 1; i < n-1; i++ {
		out = append(out, join(i)...)
	}
	return append(out, pts[n-1].Add(rightOf(pts[n-1].Sub(pts[n-2]))))
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
func inLane(path []city.Point, share float64) (city.Point, city.Point) {
	if len(path) == 0 {
		return city.Point{}, city.Point{}
	}
	lane := keepRight(path)
	return pointAlong(lane, min(1, max(0, share))*pathLength(lane))
}

// carPlace is where a traffic car phase of the way round its street, there
// and back, stands.
func carPlace(path []city.Point, phase float64) (city.Point, city.Point) {
	loop := keepRightLoop(thereAndBack(path))
	return pointAlong(loop, phase*pathLength(loop))
}

// messagePlace is where a message's van stands; a carriage on the rails
// keeps to the line.
func messagePlace(m city.Message, now time.Time) (city.Point, city.Point) {
	if m.Train {
		return m.At(now), m.At(now.Add(100 * time.Millisecond)).Sub(m.At(now))
	}
	share := m.Progress(now)
	if m.Fades {
		share = 1
	}
	return inLane(m.Path, share)
}

// deliveryPlace is where the delivery truck stands.
func deliveryPlace(d city.Delivery, now time.Time) (city.Point, city.Point) {
	share := d.Progress(now)
	if d.Fades {
		share = 0
	}
	return inLane(d.Path, share)
}

// haulPlace is where the depot's truck stands on its way.
func haulPlace(h city.Haul, now time.Time) (city.Point, city.Point) {
	dir := h.To.Sub(h.From)
	return h.At(now).Add(rightOf(dir)), dir
}

// jobPlace is where a site truck Along of its job stands: from 0 at the
// street to 1 at its stop on the way in, and on to 2 at the street on
// the way out.
func jobPlace(in, out []city.Point, along float64) (city.Point, city.Point) {
	trip := keepRight(append(append([]city.Point{}, in...), out...))
	if len(trip) < 2 {
		return inLane(in, 0)
	}
	inLen := pathLength(keepRight(in))
	dist := min(1, max(0, along)) * inLen
	if along > 1 {
		dist = inLen + min(1, along-1)*(pathLength(trip)-inLen)
	}
	return pointAlong(trip, dist)
}

// inspectorPlace is where the inspector's car stands: driving in along +x
// to park at the kerb, and out by turning back across the road.
func inspectorPlace(in city.Inspector) (city.Point, city.Point) {
	from := city.Point{X: in.Park.X - city.InspectorRun, Y: in.Park.Y}
	loop := keepRightLoop(thereAndBack([]city.Point{from, in.Park}))
	dist := in.Parked * city.InspectorRun
	if in.Leaving {
		dist = city.InspectorRun + (1-in.Parked)*(city.InspectorRun+2*laneOffset)
	}
	return pointAlong(loop, dist)
}
