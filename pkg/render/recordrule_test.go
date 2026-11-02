package render

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestRecordRule(t *testing.T) {
	cmd := int64(unsafe.Sizeof(drawCmd{}))
	game := func(liveLen, liveCap, keptLen, keptCap int) *Game {
		g := &Game{}
		g.recorder().cmds = make([]drawCmd, liveLen, liveCap)
		g.still.kept.cmds = make([]drawCmd, keptLen, keptCap)
		return g
	}

	t.Run("when both halves hold within 4× their largest use over the last 8 frames", func(t *testing.T) {
		g := game(100, 300, 100, 300)
		g.noteRecordRule()
		slack, broken := g.recordReading()

		t.Run("it should report their slack as capacity minus use, both halves", func(t *testing.T) {
			assert.Equal(t, 400*cmd, slack)
		})

		t.Run("it should name no broken half", func(t *testing.T) {
			assert.Empty(t, broken)
		})
	})

	t.Run("when a half holds over 4× its largest use for one frame, as just after a swap", func(t *testing.T) {
		g := game(10, 1000, 10, 40)
		g.noteRecordRule()
		_, once := g.recordReading()

		t.Run("it should name no broken half", func(t *testing.T) {
			assert.Empty(t, once)
		})

		t.Run("and still over the next frame", func(t *testing.T) {
			g.noteRecordRule()
			_, twice := g.recordReading()

			t.Run("it should name that half and its buffer", func(t *testing.T) {
				assert.Equal(t, "live/cmds", twice)
			})
		})
	})

	t.Run("when a buffer's largest use is in the window rather than this frame", func(t *testing.T) {
		g := game(10, 1000, 10, 40)
		g.still.sizes[0].used[3] = 300
		g.noteRecordRule()
		g.noteRecordRule()
		_, broken := g.recordReading()

		t.Run("it should hold the buffer to 4× that use", func(t *testing.T) {
			assert.Empty(t, broken)
		})
	})
}
