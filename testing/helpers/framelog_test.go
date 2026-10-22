package helpers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameLog(t *testing.T) {
	cmd := func(dst string, indices string) string {
		return "  draw-triangles: dst: " + dst + " <- src: [5, (nil), (nil), (nil)], num of dst regions: 1, num of indices: " + indices + ", blend: (source-over), shader: 1\n"
	}

	t.Run("when a log holds two frames", func(t *testing.T) {
		log := "----\nUpdate count per frame: 1\nGraphics commands:\n" +
			cmd("3", "6") + cmd("3", "12") + cmd("7 (screen)", "6") + cmd("3", "6") +
			"----\nGraphics commands:\n" + cmd("4", "600")
		frames := ParseFrameLog(strings.NewReader(log))
		require.Len(t, frames, 2)

		t.Run("it should count each frame's draw commands", func(t *testing.T) {
			assert.Equal(t, []int{4, 1}, []int{frames[0].Commands, frames[1].Commands})
		})

		t.Run("it should sum each frame's indices", func(t *testing.T) {
			assert.Equal(t, []int{30, 600}, []int{frames[0].Indices, frames[1].Indices})
		})

		t.Run("it should count a switch each time the destination changes", func(t *testing.T) {
			assert.Equal(t, []int{3, 1}, []int{frames[0].Switches, frames[1].Switches})
		})
	})

	t.Run("when commands come before the first frame", func(t *testing.T) {
		frames := ParseFrameLog(strings.NewReader(cmd("3", "6") + "----\n" + cmd("3", "6")))

		t.Run("it should not count them", func(t *testing.T) {
			assert.Equal(t, []Frame{{Commands: 1, Indices: 6, Switches: 1}}, frames)
		})
	})

	t.Run("when a command carries a first-caller line", func(t *testing.T) {
		frames := ParseFrameLog(strings.NewReader("----\n" + cmd("3", "6") + "    first-caller: pkg/render/kits.go:101\n"))

		t.Run("it should not count it as a command", func(t *testing.T) {
			assert.Equal(t, []Frame{{Commands: 1, Indices: 6, Switches: 1}}, frames)
		})
	})

	t.Run("when a frame has no commands", func(t *testing.T) {
		frames := ParseFrameLog(strings.NewReader("----\nUpdate count per frame: 1\n"))

		t.Run("it should report zeros", func(t *testing.T) {
			assert.Equal(t, []Frame{{}}, frames)
		})
	})

	// Item 162: frames the window spends putting its art up come before
	// the city's first, and how many there are moves with the art.
	t.Run("when frames come before the first the city draws", func(t *testing.T) {
		still := "still: composes=0 redos=0 overlays=0 rects=0 ground=1 direct=1 shifts=0\n"
		frames := ParseFrameLog(strings.NewReader("----\n" + cmd("3", "6") + "----\n" + cmd("3", "6") +
			"----\n" + cmd("4", "60") + still + "----\n" + cmd("4", "90")))
		city := CityFrames(frames)

		t.Run("it should keep the city's frames from its first", func(t *testing.T) {
			assert.Equal(t, []int{60, 90}, []int{city[0].Indices, city[1].Indices})
		})

		t.Run("it should drop the frames before it", func(t *testing.T) {
			assert.Len(t, city, 2)
		})
	})

	t.Run("when the city draws no frame", func(t *testing.T) {
		t.Run("it should keep none", func(t *testing.T) {
			assert.Empty(t, CityFrames(ParseFrameLog(strings.NewReader("----\n"+cmd("3", "6")))))
		})
	})

	t.Run("when the worst of several frames is asked for", func(t *testing.T) {
		worst := MostWork([]Frame{{Commands: 5, Indices: 10, Switches: 1}, {Commands: 2, Indices: 40, Switches: 3}})

		t.Run("it should take each count's largest on its own", func(t *testing.T) {
			assert.Equal(t, Frame{Commands: 5, Indices: 40, Switches: 3}, worst)
		})
	})
}

// Item 192: the city's debug build reports what its look cache did each
// frame beside Ebitengine's commands.
func TestFrameLogLooks(t *testing.T) {
	t.Run("when a frame reports its looks", func(t *testing.T) {
		frames := ParseFrameLog(strings.NewReader("----\nlooks: hits=40 misses=3 bakes=2 bytes=1048576\n"))
		require.Len(t, frames, 1)

		t.Run("it should read what the cache did", func(t *testing.T) {
			assert.Equal(t, Looks{Hits: 40, Misses: 3, Bakes: 2, Bytes: 1 << 20}, frames[0].Looks)
		})
	})

	t.Run("when the worst of several frames is asked for", func(t *testing.T) {
		worst := MostWork([]Frame{{Looks: Looks{Misses: 5, Bakes: 1, Bytes: 10}}, {Looks: Looks{Misses: 2, Bakes: 4, Bytes: 30}}})

		t.Run("it should take each look count's largest", func(t *testing.T) {
			assert.Equal(t, Looks{Misses: 5, Bakes: 4, Bytes: 30}, worst.Looks)
		})
	})
}

// Item 159: the city's debug build reports the footer's reading line
// each frame it shows one.
func TestFrameLogReading(t *testing.T) {
	t.Run("when the second of two frames reports a reading line", func(t *testing.T) {
		frames := ParseFrameLog(strings.NewReader("----\n----\nreading: Reading your history · 42% · statistics fill in as it reads\n"))
		require.Len(t, frames, 2)

		t.Run("it should read the line on that frame and none on the other", func(t *testing.T) {
			assert.Equal(t, []string{"", "Reading your history · 42% · statistics fill in as it reads"}, []string{frames[0].Reading, frames[1].Reading})
		})
	})
}

// Item 193: the city's debug build reports what its standing city cost
// each frame: composed whole, composed again in rectangles, the group
// lines composed, rectangles of movers drawn again, and the ground
// composed.
func TestFrameLogStill(t *testing.T) {
	t.Run("when a frame reports its standing city", func(t *testing.T) {
		frames := ParseFrameLog(strings.NewReader("----\nstill: composes=1 redos=2 overlays=1 rects=11 ground=1 direct=1 shifts=3\n"))
		require.Len(t, frames, 1)

		t.Run("it should read what the standing city cost", func(t *testing.T) {
			assert.Equal(t, Still{Composes: 1, Redos: 2, Overlays: 1, Rects: 11, Ground: 1, Direct: 1, Shifts: 3}, frames[0].Still)
		})
	})

	t.Run("when the worst of several frames is asked for", func(t *testing.T) {
		worst := MostWork([]Frame{{Still: Still{Composes: 1, Rects: 3}}, {Still: Still{Redos: 2, Rects: 9, Ground: 1}}})

		t.Run("it should take each count's largest", func(t *testing.T) {
			assert.Equal(t, Still{Composes: 1, Redos: 2, Rects: 9, Ground: 1}, worst.Still)
		})
	})
}
