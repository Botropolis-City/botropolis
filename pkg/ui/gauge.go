package ui

import "botropolis.city/botropolis/pkg/city"

// The usage gauges' layout. Bug 52: the percentage is read *along* the
// river and the boats are separated *across* it, which is the way round
// Aria asked for — "going bottom to top and face that direction", and
// "spread out left to right as well". Item 49 had the two swapped, so
// the reading sat on the river's shortest dimension and the long axis
// carried nothing.
//
// Both are pure arithmetic on a width and a count, so they live here
// and pkg/render decides where on the water that lands.

// GaugeLanes are the offsets from the river's centre line at which
// boats ride, one lane each, so that boats with equal readings are
// still told apart. beams is each boat's width in lane order, and
// margin is the water left between a hull and the bank.
//
// The outer lanes are pinned by their OWN half beam plus the margin,
// not by the widest hull's. Bug 55: reserving the widest at both sides
// put the widest hull exactly half a beam from the bank — a clearance
// of zero, arrived at by construction rather than chosen by anyone, so
// the liner's superstructure sat over the grass. Pinning each end by
// the hull that actually rides there recovers the room the narrow boat
// was never using, which is what pays for the margin.
func GaugeLanes(width float64, beams []float64, margin float64) []float64 {
	if len(beams) == 0 {
		return nil
	}
	if len(beams) == 1 {
		return []float64{0}
	}
	lo := -width/2 + margin + beams[0]/2
	hi := width/2 - margin - beams[len(beams)-1]/2
	if hi <= lo {
		// Narrower water than the hulls need. Still give every boat a
		// lane: the reading has to be drawn, and the guard that the
		// river is wide enough lives with the river.
		lo, hi = 0, 0
	}
	step := (hi - lo) / float64(len(beams)-1)
	lanes := make([]float64, len(beams))
	for i := range lanes {
		lanes[i] = lo + step*float64(i)
	}
	return lanes
}

// GaugeShare is how far along the run a reading sits; see
// city.GaugeShare, which the moving boats need too.
func GaugeShare(percent float64) float64 { return city.GaugeShare(percent) }
