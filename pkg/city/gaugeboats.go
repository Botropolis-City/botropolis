package city

import (
	"math"
	"time"
)

// The usage boats in motion. Item 79.
//
// A boat's place along the river is its reading, so it used to hold
// still and jump when a reading changed. Aria: "when things change, the
// boats should drive forward instead of teleporting." And with the
// probe left to the user: "we should have the boats not there by
// default. We should have them drive up when you update manually and
// then drive off the screen."
//
// So every boat has a track, and moves along it at one speed and only
// ever forward. 0 is the empty end of the run and 1 the limit; below 0
// is downriver of the run and above 1 upriver of it, and GaugeOffLow
// and GaugeOffHigh are far enough out to be off the map. A reading that
// falls — a window turning over — does not back the boat down the
// river: it carries on off the top and comes round from the bottom.

const (
	// GaugeOffLow and GaugeOffHigh are where a boat is out of sight:
	// a whole run beyond either end, which the renderer stretches to
	// reach the river's ends.
	GaugeOffLow  = -1.0
	GaugeOffHigh = 2.0

	// GaugeRunFor is how long a boat takes to drive one whole run.
	// Slower than the courier, which crosses the entire river in
	// CourierFor: these are cargo, and the courier has to visibly
	// outrun them.
	GaugeRunFor = 3 * time.Second

	// GaugeDriveMin keeps a small change visible as a drive: a reading
	// that moves two points would otherwise cover the distance in a
	// tenth of a second, which reads as a jump.
	GaugeDriveMin = time.Second

	// GaugeStayFor is how long boats called up by hand stay on station
	// before they sail off: long enough to point at each one.
	GaugeStayFor = time.Minute
)

// GaugeBoat is one usage boat as it stands now.
type GaugeBoat struct {
	Gauge
	Track float64
}

type gaugeLeg struct {
	from, to   float64
	start, end time.Time
}

func (l gaugeLeg) at(now time.Time) float64 {
	if !now.After(l.start) {
		return l.from
	}
	if !now.Before(l.end) {
		return l.to
	}
	p := float64(now.Sub(l.start)) / float64(l.end.Sub(l.start))
	p = p * p * (3 - 2*p)
	return l.from + (l.to-l.from)*p
}

type gaugeBoat struct {
	gauge Gauge
	rest  float64
	legs  []gaugeLeg
}

// settle folds the legs that have finished into where the boat rests.
// A boat that has driven off the top rests off the bottom, ready to
// come in the way every boat does.
func (b *gaugeBoat) settle(now time.Time) {
	for len(b.legs) > 0 && !now.Before(b.legs[0].end) {
		b.rest = b.legs[0].to
		b.legs = b.legs[1:]
	}
	if len(b.legs) == 0 && b.rest >= GaugeOffHigh {
		b.rest = GaugeOffLow
	}
}

func (b *gaugeBoat) track(now time.Time) float64 {
	b.settle(now)
	if len(b.legs) > 0 {
		return b.legs[0].at(now)
	}
	return b.rest
}

func (b *gaugeBoat) away() bool {
	return len(b.legs) == 0 && b.rest <= GaugeOffLow
}

func (b *gaugeBoat) finished() time.Time {
	if len(b.legs) == 0 {
		return time.Time{}
	}
	return b.legs[len(b.legs)-1].end
}

// GaugeShare is how far along the run a reading sits, from 0 at the
// near end to 1 at the limit.
//
// A reading past its limit holds at the far end rather than carrying on
// upstream. The boat is then level with the flagged buoy, which is the
// legible way to say "at or past it" without needing the number.
func GaugeShare(percent float64) float64 {
	return min(1, max(0, percent/100))
}

// driveTo sets a boat going from wherever it is now to target, forward
// only: a target behind it is reached by way of the top of the map.
func (s *Scene) driveTo(b *gaugeBoat, target float64, now time.Time) {
	from := b.track(now)
	b.rest, b.legs = from, nil
	if s.reduced {
		b.rest = target
		b.settle(now)
		return
	}
	stops := []float64{target}
	if target < from {
		stops = []float64{GaugeOffHigh, GaugeOffLow, target}
	}
	at := now
	for _, to := range stops {
		if to == GaugeOffLow && from == GaugeOffHigh {
			from = GaugeOffLow
			continue
		}
		if to == from {
			continue
		}
		dur := max(GaugeDriveMin, time.Duration(math.Abs(to-from)*float64(GaugeRunFor)))
		b.legs = append(b.legs, gaugeLeg{from: from, to: to, start: at, end: at.Add(dur)})
		at, from = at.Add(dur), to
	}
}

// SetGaugesManual says whether the boats come only when called. Turned
// on, the boats on the river sail off; turned off, the boats that are
// away come in to stay.
func (s *Scene) SetGaugesManual(manual bool) {
	if s.gaugesManual == manual {
		return
	}
	s.gaugesManual = manual
	s.gaugeLeaveAt = time.Time{}
	now := s.now()
	for _, b := range s.gaugeBoats {
		if manual {
			s.driveTo(b, GaugeOffHigh, now)
		} else {
			s.driveTo(b, GaugeShare(b.gauge.Percent), now)
		}
	}
}

// SetGauges takes a fresh reading. A boat on the river drives to it; a
// boat that is away takes note and stays away.
func (s *Scene) SetGauges(gauges []Gauge) {
	now := s.now()
	if s.gaugeBoats == nil {
		s.gaugeBoats = map[string]*gaugeBoat{}
	}
	seen := map[string]bool{}
	s.gaugeOrder = s.gaugeOrder[:0]
	for _, g := range gauges {
		seen[g.Title] = true
		s.gaugeOrder = append(s.gaugeOrder, g.Title)
		target := GaugeShare(g.Percent)
		b, ok := s.gaugeBoats[g.Title]
		if !ok {
			b = &gaugeBoat{gauge: g, rest: GaugeOffLow}
			s.gaugeBoats[g.Title] = b
			if !s.gaugesManual {
				// The first reading places the boats rather than sailing
				// them in: a still or a film's first frame is taken as
				// the first snapshot lands.
				b.rest = target
				continue
			}
		}
		b.gauge = g
		if b.away() && s.gaugesManual && s.gaugeLeaveAt.IsZero() {
			continue
		}
		if !b.leaving() {
			s.driveTo(b, target, now)
		}
	}
	for title := range s.gaugeBoats {
		if !seen[title] {
			delete(s.gaugeBoats, title)
		}
	}
}

func (b *gaugeBoat) leaving() bool {
	return len(b.legs) > 0 && b.legs[len(b.legs)-1].to == GaugeOffHigh
}

// VisitGauges calls the boats up: they sail in to their readings, stay
// for GaugeStayFor, and sail off. Calling again while they are out
// starts the stay over.
func (s *Scene) VisitGauges() {
	now := s.now()
	arrived := now
	for _, b := range s.gaugeBoats {
		s.driveTo(b, GaugeShare(b.gauge.Percent), now)
		if end := b.finished(); end.After(arrived) {
			arrived = end
		}
	}
	s.gaugeLeaveAt = arrived.Add(GaugeStayFor)
}

// GaugeBoats is every boat on the river now, in reading order.
func (s *Scene) GaugeBoats() []GaugeBoat {
	now := s.now()
	if s.gaugesManual && !s.gaugeLeaveAt.IsZero() && !now.Before(s.gaugeLeaveAt) {
		// From when the stay ended rather than from this frame: nothing
		// may have been drawn for a while, and the boats left on time.
		leave := s.gaugeLeaveAt
		s.gaugeLeaveAt = time.Time{}
		for _, b := range s.gaugeBoats {
			s.driveTo(b, GaugeOffHigh, leave)
		}
	}
	var out []GaugeBoat
	for _, title := range s.gaugeOrder {
		b, ok := s.gaugeBoats[title]
		if !ok {
			continue
		}
		track := b.track(now)
		if track <= GaugeOffLow || track >= GaugeOffHigh {
			continue
		}
		out = append(out, GaugeBoat{Gauge: b.gauge, Track: track})
	}
	return out
}

// gaugesMoving reports whether any boat is mid-drive.
func (s *Scene) gaugesMoving() bool {
	now := s.now()
	for _, b := range s.gaugeBoats {
		if len(b.legs) > 0 && now.Before(b.finished()) {
			return true
		}
	}
	return false
}
