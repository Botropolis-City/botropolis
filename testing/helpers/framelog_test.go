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

	t.Run("when the worst of several frames is asked for", func(t *testing.T) {
		worst := MostWork([]Frame{{Commands: 5, Indices: 10, Switches: 1}, {Commands: 2, Indices: 40, Switches: 3}})

		t.Run("it should take each count's largest on its own", func(t *testing.T) {
			assert.Equal(t, Frame{Commands: 5, Indices: 40, Switches: 3}, worst)
		})
	})
}
