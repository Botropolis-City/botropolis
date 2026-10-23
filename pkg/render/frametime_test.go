package render

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Item 201: a city nobody is watching says, with the window's state,
// what its snapshots have cost it, so that cost can be gated apart from
// drawing, which a virtual display does in software.
func TestQuietLine(t *testing.T) {
	t.Run("when the frame timer is on and snapshots have been laid out", func(t *testing.T) {
		var out bytes.Buffer
		f := &frameTimer{on: true, out: &out, reading: func() time.Duration { return 30 * time.Millisecond }, received: func() int64 { return 7168 }}
		f.laidOut(12 * time.Millisecond)
		f.laidOut(12 * time.Millisecond)
		f.quiet(false, true, false)

		t.Run("it should say how many, the time spent decoding and laying them out, and the bytes read from the daemon", func(t *testing.T) {
			assert.Contains(t, out.String(), "QUIET no frames: focused=false visible=true minimised=false snapshots=2 decoding=30.0ms layout=24.0ms received=7168B\n")
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
