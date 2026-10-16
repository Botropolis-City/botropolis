package render

import (
	"time"

	"botropolis.city/botropolis/pkg/city"
)

// motionClock is two clocks the map animates to. Item 89.
//
// Attention runs at the clock's own rate whatever the liveliness, so
// something that wants you pulses at every level. Ambient runs at the
// level's pace, and is integrated rather than multiplied, so changing
// the level changes how fast the movers go from where they are instead
// of moving them all at once.
type motionClock struct {
	last      time.Time
	ambient   float64
	attention float64
}

func newMotionClock(start time.Time) motionClock {
	return motionClock{last: start}
}

// step moves both clocks on to now, at the given level.
func (m *motionClock) step(now time.Time, level city.Liveliness) {
	dt := now.Sub(m.last).Seconds()
	m.last = now
	if dt <= 0 {
		return
	}
	m.attention += dt
	m.ambient += dt * level.Pace()
}
