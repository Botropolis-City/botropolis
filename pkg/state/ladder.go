package state

import "time"

// Rung is how far a session has gone quiet (item 86). A stopped session
// stays on its lot and climbs the ladder with age; time never takes it
// off the map.
type Rung string

const (
	RungLive      Rung = "live"
	RungResting   Rung = "resting"
	RungShuttered Rung = "shuttered"
	RungOvergrown Rung = "overgrown"
	// RungPacked is a session you packed up, off the map in the depot.
	RungPacked Rung = "packed up"
)

// Thresholds are how long a stopped session stays quiet before it is
// shuttered, and before it is overgrown.
type Thresholds struct {
	Shuttered time.Duration
	Overgrown time.Duration
}

var DefaultThresholds = Thresholds{Shuttered: 7 * 24 * time.Hour, Overgrown: 30 * 24 * time.Hour}

// Rung places the session on the ladder. A last activity that is unknown
// or ahead of the clock ages nothing.
func (s Session) Rung(now time.Time, t Thresholds) Rung {
	if s.Packed {
		return RungPacked
	}
	if s.State != Resting {
		return RungLive
	}
	if s.LastActivity.IsZero() || s.LastActivity.After(now) {
		return RungResting
	}
	switch quiet := now.Sub(s.LastActivity); {
	case quiet >= t.Overgrown:
		return RungOvergrown
	case quiet >= t.Shuttered:
		return RungShuttered
	}
	return RungResting
}
