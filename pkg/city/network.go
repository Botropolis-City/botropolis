package city

// The networks: what the map draws *between* objects rather than on
// them. Wires from the plant, beams from the towers, cars on the
// avenues, the freight loop, the cranes over a building.
//
// Drawing all of them at once is why the map reads busy, and it is also
// most of what a frame costs — the networks are the movers, and the
// movers are what is left after the still city is composed once. Putting
// each behind the view that asks about it is the same decision twice:
// the map says less at a glance, and Attention stops paying for four
// answers nobody asked for.

// Network is one of them.
type Network int

const (
	// NetworkWires is the plant's line to each session: it carries the
	// token rate, and its absence carries "the daemon has never heard
	// from this session", which is why Health draws them too.
	NetworkWires Network = iota
	// NetworkFreight is the token train round its loop.
	NetworkFreight
	// NetworkBeams is a tower's line to the sessions calling that server.
	NetworkBeams
	// NetworkTraffic is the cars between repos.
	NetworkTraffic
	// NetworkCranes is the subagents standing over a building.
	NetworkCranes
)

// Networks is every one, for walking them.
var Networks = []Network{NetworkWires, NetworkFreight, NetworkBeams, NetworkTraffic, NetworkCranes}

func (n Network) String() string {
	switch n {
	case NetworkWires:
		return "wires"
	case NetworkFreight:
		return "freight"
	case NetworkBeams:
		return "beams"
	case NetworkTraffic:
		return "traffic"
	case NetworkCranes:
		return "cranes"
	}
	return "unknown"
}

// shown is which networks each view draws. A view not in the map draws
// none, which is the answer for Attention and for every view that counts
// something the networks do not carry.
var shown = map[View][]Network{
	// The base view draws traffic and nothing else. Bug 48: Aria asked
	// for cars on the main view, and the reason it is safe to break the
	// subtractive rule here is that a car already answers for itself —
	// its hover card names the two projects and the messages, files and
	// sessions passing between them, which is what section 10's item 5
	// asked for. Unexplained motion would be the complaint that started
	// this project; motion that says what it is on hover is not.
	//
	// A drone answers for itself the same way, so the drones show
	// wherever the cars do (item 89).
	ViewAttention: {NetworkTraffic, NetworkCranes},
	ViewSpend:     {NetworkWires, NetworkFreight},
	ViewServers:   {NetworkBeams},
	ViewTraffic:   {NetworkTraffic, NetworkCranes},
	ViewFanout:    {NetworkCranes},
	// Health draws the wires for what is *not* there. A session the
	// daemon has seen no hook events from has no wire to the plant, and
	// that is a health signal rather than a spend one — it means the
	// numbers beside it are files-only guesses. It would have vanished
	// with the wires otherwise.
	ViewHealth: {NetworkWires},
}

// Shows reports whether this view draws that network.
func (v View) Shows(n Network) bool {
	for _, have := range shown[v] {
		if have == n {
			return true
		}
	}
	return false
}

// carried is where one carrier sits against the busiest of its kind, so
// a wire, a beam or a road is painted by what it carries rather than
// only by what it connects. A view that does not own the network has
// nothing to say about it.
func (s *Scene) carried(n Network, value, top float64) Tint {
	if !s.view.Shows(n) {
		return Tint{}
	}
	if top <= 0 {
		return Tint{Known: true}
	}
	return Tint{Value: min(1, value/top), Known: true}
}

// WireTint is where a power line sits on the spend ramp, against the
// busiest wire on the map.
func (s *Scene) WireTint(l PowerLine, all []PowerLine) Tint {
	top := 0.0
	for _, other := range all {
		if v := wireLoad(other); v > top {
			top = v
		}
	}
	return s.carried(NetworkWires, wireLoad(l), top)
}

func wireLoad(l PowerLine) float64 { return l.Fresh + l.Cached }

// BeamTint is where a beam sits, against the busiest beam.
func (s *Scene) BeamTint(b Beam, all []Beam) Tint {
	top := 0.0
	for _, other := range all {
		if v := float64(other.Calls); v > top {
			top = v
		}
	}
	return s.carried(NetworkBeams, float64(b.Calls), top)
}

// RoadTint is where a road sits, against the busiest road.
func (s *Scene) RoadTint(r RoadLine, all []RoadLine) Tint {
	top := 0.0
	for _, other := range all {
		if v := roadLoad(other); v > top {
			top = v
		}
	}
	return s.carried(NetworkTraffic, roadLoad(r), top)
}

func roadLoad(r RoadLine) float64 { return float64(r.Messages + r.Files) }
