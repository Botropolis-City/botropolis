package render

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/measure"
)

// Item 201: a city nobody is watching says, with the window's state,
// what its snapshots have cost it, so that cost can be gated apart from
// drawing, which a virtual display does in software.
func TestQuietLine(t *testing.T) {
	quiet := func(decoding, layout measure.Cost) string {
		var out bytes.Buffer
		f := &frameTimer{on: true, out: &out, reading: func() measure.Cost { return decoding }, received: func() int64 { return 7168 }}
		f.laidOut(layout)
		f.laidOut(layout)
		f.quiet(false, true, false)
		return out.String()
	}

	t.Run("when the frame timer is on and snapshots have been laid out, timed on the thread's CPU", func(t *testing.T) {
		said := quiet(measure.Cost{CPU: 30 * time.Millisecond}, measure.Cost{CPU: 12 * time.Millisecond})

		t.Run("it should say how many, the time spent decoding and laying them out, the bytes read from the daemon, and that they were not counted", func(t *testing.T) {
			assert.Contains(t, said, "QUIET no frames: focused=false visible=true minimised=false snapshots=2 decoding=30.0ms layout=24.0ms received=7168B counted=thread-cpu\n")
		})
	})

	// Item 209: counted, they are the time a core takes to retire them.
	t.Run("when their instructions were counted", func(t *testing.T) {
		at := func(d time.Duration) uint64 { return uint64(d.Seconds() * measure.CoreRate) }
		said := quiet(measure.Cost{CPU: time.Second, Instructions: at(30 * time.Millisecond), Counted: true},
			measure.Cost{CPU: time.Second, Instructions: at(12 * time.Millisecond), Counted: true})

		t.Run("it should give them as a core's time, and say they were counted", func(t *testing.T) {
			assert.Contains(t, said, "decoding=30.0ms layout=24.0ms received=7168B counted=instructions\n")
		})
	})
}

// Item 209: a city that draws says so at its first frame, so a test can
// tell it is running without waiting for 120 frames, which a virtual
// display draws in software and a busy machine stretches past a minute.
func TestFirstFrame(t *testing.T) {
	t.Run("when the frame timer is on and two frames are drawn", func(t *testing.T) {
		var out bytes.Buffer
		f := &frameTimer{on: true, out: &out}
		f.start()()
		f.start()()

		t.Run("it should say once that the first frame was drawn", func(t *testing.T) {
			assert.Equal(t, "DRAWN the first frame\n", out.String())
		})
	})
}
