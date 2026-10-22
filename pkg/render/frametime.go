package render

import (
	"fmt"
	"io"
	"os"
	"time"

	"botropolis.city/botropolis/pkg/proto"
)

// What a frame costs the CPU, and how often the static layer is reused.
//
// This is the number to optimise against, and it needs saying why. A
// virtual display draws in software, so measuring the process there
// measures fill rate — how many pixels Mesa shaded — which caching does
// not change: one full-screen blit shades as much as the sprites it
// replaces. On a real graphics card the pixels are free and the cost is
// the work of issuing the drawing, which is what this counts.
//
// Set BOTROPOLIS_FRAMETIME=1 and it prints a line every reportEvery
// frames to stderr.
const reportEvery = 120

type frameTimer struct {
	on         bool
	frames     int
	draw       time.Duration
	hits       int
	misses     int
	lastQuiet  time.Time
	lastTick   time.Time
	lastFrames int
	// out is where it reports; nil is stderr.
	out io.Writer
	// snapshots and layout are how many snapshots have been laid out and
	// the time it took, and reading is the time spent decoding them, so
	// a city that draws nothing can be gated on them apart from drawing,
	// which a virtual display does in software (item 201).
	snapshots int
	layout    time.Duration
	reading   func() time.Duration
}

var frames = &frameTimer{on: os.Getenv("BOTROPOLIS_FRAMETIME") != "", reading: proto.Reading}

func (f *frameTimer) writer() io.Writer {
	if f.out == nil {
		return os.Stderr
	}
	return f.out
}

// laidOut records a snapshot laid out, and how long it took.
func (f *frameTimer) laidOut(took time.Duration) {
	f.snapshots++
	f.layout += took
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

// Why a run draws nothing.
//
// A window that paints no frames still reports a plausible-looking CPU
// figure — a quarter of one core, spent sleeping — and the only symptom
// is a number that means "an app that was not running" while looking
// like a measurement. Three of eight runs on one desk did exactly that.
// So there are two reports, because there are two ways to draw nothing
// and they need telling apart.

// quiet is the first: the window is there and the loop is running, but
// nobody is watching it, and it says which of the three conditions
// failed.
func (f *frameTimer) quiet(focused, visible, minimised bool) {
	if !f.on {
		return
	}
	if now := time.Now(); now.Sub(f.lastQuiet) >= time.Second {
		f.lastQuiet = now
		fmt.Fprintf(f.writer(), "QUIET no frames: focused=%v visible=%v minimised=%v snapshots=%d decoding=%.1fms layout=%.1fms\n",
			focused, visible, minimised, f.snapshots, ms(f.reading()), ms(f.layout))
	}
}

// tick is the second, and the one that catches the harder case: Update
// is running and Draw is not being called at all, so neither the frame
// counter nor the gate above ever gets a word in. Silence then looks
// exactly like an idle app, which is how a scripted window on a real
// desktop can sit focused and visible and paint nothing for a minute
// while every probe says it should be drawing.
func (f *frameTimer) tick() {
	if !f.on {
		return
	}
	now := time.Now()
	if f.lastTick.IsZero() {
		f.lastTick, f.lastFrames = now, f.frames
		return
	}
	if now.Sub(f.lastTick) < 5*time.Second {
		return
	}
	if f.frames == f.lastFrames {
		fmt.Fprintf(f.writer(), "QUIET update is running but Draw has painted nothing for %.0fs\n", now.Sub(f.lastTick).Seconds())
	}
	f.lastTick, f.lastFrames = now, f.frames
}

// note records whether the static layer was reused this frame.
func (f *frameTimer) note(hit bool) {
	if !f.on {
		return
	}
	if hit {
		f.hits++
	} else {
		f.misses++
	}
}

// start opens a frame; the returned function closes it.
func (f *frameTimer) start() func() {
	if !f.on {
		return func() {}
	}
	at := time.Now()
	return func() {
		f.draw += time.Since(at)
		f.frames++
		if f.frames%reportEvery != 0 {
			return
		}
		fmt.Fprintf(f.writer(), "FRAME %d frames: %.2f ms/frame of drawing, static layer reused %d of %d\n",
			f.frames, float64(f.draw.Microseconds())/1000/float64(reportEvery), f.hits, f.hits+f.misses)
		f.draw, f.hits, f.misses = 0, 0, 0
	}
}
