package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// step is one part of a building's drawing, in the order it is painted.
type step uint8

const (
	stepCourses step = iota
	stepSprite
	stepSite
	stepFacade
	stepLights
	stepBeacon
	stepSign
	stepFlags
	stepCelebrate
	stepSmoke
	stepSelected
	stepSkip
	stepSkipTrucks
	stepWork
	stepSpire
	stepCount
)

// lookState is what decides which of a building's steps it draws this
// frame and which of them are moving.
type lookState struct {
	courses, pouring                 bool
	rising, fading, settling         bool
	building                         bool
	stop, dimmed                     bool
	facade                           bool
	lights                           bool
	beacon, pulse                    bool
	sign                             bool
	flags                            int
	hoisting                         bool
	celebrating, smoke               bool
	selected                         bool
	skip, trucks, work, loads, spire bool
}

// steps is which steps a building draws, and which of those move: a
// moving step is drawn every frame, and the rest can be baked.
func (s lookState) steps() (present, live [stepCount]bool) {
	present[stepCourses], live[stepCourses] = s.courses, s.pouring
	present[stepSprite], live[stepSprite] = true, s.rising || s.fading || s.settling || s.building
	present[stepSite], live[stepSite] = s.building, true
	if !s.stop {
		present[stepFacade], live[stepFacade] = s.facade || s.settling, s.settling
		present[stepLights], live[stepLights] = s.lights, true
		present[stepBeacon], live[stepBeacon] = s.beacon || s.settling, s.pulse || s.settling
		present[stepSign] = s.sign
		present[stepFlags], live[stepFlags] = s.flags > 0, s.hoisting
		present[stepCelebrate], live[stepCelebrate] = s.celebrating, true
		present[stepSmoke], live[stepSmoke] = s.smoke, true
		present[stepSelected] = s.selected
	}
	if !s.dimmed {
		present[stepSkip] = s.skip
		present[stepSkipTrucks], live[stepSkipTrucks] = s.trucks, true
		present[stepWork], live[stepWork] = s.work || s.loads, s.loads
		present[stepSpire] = s.spire
	}
	return present, live
}

// run is a stretch of steps drawn together: baked into one image, or
// drawn straight onto the frame.
type run struct {
	from, to step
	baked    bool
}

// runsOf groups a building's steps into runs: each stretch of steps that
// do not move is one baked run, and the steps that do, and its lights,
// stand between them. Together the runs cover every step, in order.
func runsOf(present, live [stepCount]bool) []run {
	var out []run
	for i := step(0); i < stepCount; i++ {
		if !present[i] {
			continue
		}
		baked := !live[i]
		if n := len(out); n > 0 && out[n-1].baked == baked && i != stepLights && out[n-1].from != stepLights {
			continue
		}
		if n := len(out); n > 0 {
			out[n-1].to = i
		}
		out = append(out, run{from: i, to: stepCount, baked: baked})
	}
	if len(out) > 0 {
		out[0].from = 0
	}
	return out
}

// lookPhaseSteps is how finely a look's sub-pixel phase is told apart:
// a look baked at one phase is blitted at whole pixels, so it is off by
// no more than this at another inside the same step.
const lookPhaseSteps = 64

// lookPhase is where a building's foot falls inside its pixel.
func lookPhase(foot city.Point) [2]int {
	frac := func(v float64) int {
		return int(math.Round((v-math.Floor(v))*lookPhaseSteps)) % lookPhaseSteps
	}
	return [2]int{frac(foot.X), frac(foot.Y)}
}

// lookKey is everything a baked run of a building looks the way it does
// because of. While it holds, the run is a blit.
type lookKey struct {
	id      string
	run     step
	present [stepCount]bool
	live    [stepCount]bool
	atlas   *kitAtlas
	faces   *faces
	piece   string
	heading int
	zoom    float64
	phase   [2]int
	tint    ebiten.ColorScale
	tinted  bool
	recede  bool
	viewing bool
	reduced bool
	signage city.Signage
	packed  bool
	gone    bool

	compactions int
	litCourse   bool
	dark        bool
	boards      bool
	beacon      color.NRGBA
	title       string
	shown       bool
	flags       [3]flagLook
	selected    bool
	skip        string
	scaffold    float64
	spire       int
}

// flagLook is how one of a building's flags looks.
type flagLook struct {
	colour color.NRGBA
	hoist  float64
}
