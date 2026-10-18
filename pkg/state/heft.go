package state

import (
	"math"
	"slices"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

// MaxStoreys is the tallest a building grows (item 109).
const MaxStoreys = 5

// StarterCuts are the scale's cut points before the user has a history
// of their own, and what a short history blends into.
var StarterCuts = [MaxStoreys - 1]int64{50_000, 250_000, 1_000_000, 5_000_000}

// fullSample is how many sessions make a history the user's own alone.
const fullSample = 20

// ScaleLife is how long a scale stands before it is computed again.
const ScaleLife = 24 * time.Hour

// Heft is how heavy a session's work was: every token it sent and
// received, with cache reads counted at a tenth (item 109).
func Heft(u claude.Usage) int64 {
	return u.Input + u.Output + u.CacheCreate + u.CacheRead/10
}

// Scale is where a session's heft crosses into each storey: the fifths
// of the user's own sessions, blended into the starter lines while the
// history is short.
type Scale struct {
	Cuts   [MaxStoreys - 1]int64
	Sample int
	At     time.Time
}

// NewScale cuts hefts into fifths at a time.
func NewScale(hefts []int64, at time.Time) Scale {
	s := Scale{Cuts: StarterCuts, Sample: len(hefts), At: at}
	n := len(hefts)
	if n == 0 {
		return s
	}
	sorted := slices.Clone(hefts)
	slices.Sort(sorted)
	w := min(1, float64(n)/fullSample)
	for i := range s.Cuts {
		own := float64(max(1, sorted[(i+1)*n/MaxStoreys]))
		starter := float64(StarterCuts[i])
		s.Cuts[i] = int64(math.Round(math.Exp((1-w)*math.Log(starter) + w*math.Log(own))))
	}
	return s
}

// Storeys is how many storeys a heft reaches, one to MaxStoreys.
func (s Scale) Storeys(heft int64) int {
	storeys := 1
	for _, c := range s.Cuts {
		if heft >= c {
			storeys++
		}
	}
	return storeys
}

// Stale is whether the scale has stood a day.
func (s Scale) Stale(now time.Time) bool {
	return now.Sub(s.At) >= ScaleLife
}

// Heights gives every session its storeys, on one scale for every
// harness, and keeps the storeys each has reached so none ever loses
// one. It is kept in memory for the daemon's run and never written.
type Heights struct {
	scale   Scale
	own     bool
	reached map[string]int
}

func NewHeights() *Heights {
	return &Heights{scale: Scale{Cuts: StarterCuts}, reached: map[string]int{}}
}

// Scale is the scale storeys are drawn on now.
func (h *Heights) Scale() Scale {
	return h.scale
}

// Apply sets Storeys on every session. read is whether the user's whole
// history has been read, without which the scale stays on the starter
// lines and storeys may still fall.
func (h *Heights) Apply(sessions []Session, read bool, now time.Time) {
	if read && (!h.own || h.scale.Stale(now)) {
		var sample []int64
		for _, s := range sessions {
			if s.Heft > 0 && !s.HeftPartial && !s.ConversationGone {
				sample = append(sample, s.Heft)
			}
		}
		h.scale, h.own = NewScale(sample, now), true
	}
	for i := range sessions {
		s := &sessions[i]
		storeys := h.scale.Storeys(s.Heft)
		if s.ConversationGone {
			storeys = 1
		}
		if h.own {
			storeys = max(storeys, h.reached[s.ID])
			h.reached[s.ID] = storeys
		}
		s.Storeys = storeys
	}
}
