package render

import (
	"runtime"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 212: the frame's two buffers grew to the biggest frame they had
// held and never gave it back; on Aria's city a settled frame used 474
// points of a buffer sized for 136,533. A buffer used under a quarter for
// sizeWindow frames running is made twice the most it used, so none holds
// over four times what its frames use for long.
func TestTheRecordsBuffers(t *testing.T) {
	type buffer struct {
		name       string
		used, held func(l *drawList) int
		grow       func(l *drawList)
	}
	const room = 1 << 16
	buffers := []buffer{
		{"commands", func(l *drawList) int { return len(l.cmds) }, func(l *drawList) int { return cap(l.cmds) },
			func(l *drawList) { l.cmds = append(make([]drawCmd, 0, room), l.cmds...) }},
		{"vertices", func(l *drawList) int { return len(l.vertices) }, func(l *drawList) int { return cap(l.vertices) },
			func(l *drawList) { l.vertices = append(make([]ebiten.Vertex, 0, room), l.vertices...) }},
		{"indices", func(l *drawList) int { return len(l.indices) }, func(l *drawList) int { return cap(l.indices) },
			func(l *drawList) { l.indices = append(make([]uint32, 0, room), l.indices...) }},
		{"image options", func(l *drawList) int { return len(l.imgOps) }, func(l *drawList) int { return cap(l.imgOps) },
			func(l *drawList) { l.imgOps = append(make([]ebiten.DrawImageOptions, 0, room), l.imgOps...) }},
		{"triangle options", func(l *drawList) int { return len(l.triOps) }, func(l *drawList) int { return cap(l.triOps) },
			func(l *drawList) { l.triOps = append(make([]ebiten.DrawTrianglesOptions, 0, room), l.triOps...) }},
	}
	// grown is a settled city whose buffers, both halves, hold far more
	// than its frames use, as after a frame that recorded everything.
	grown := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 1
		for range 4 {
			rc.frame()
		}
		for range 2 {
			for _, b := range buffers {
				b.grow(rc.g.recorder())
			}
			rc.frame()
		}
		return rc
	}

	t.Run("when a settled city's buffers hold far more than its frames use", func(t *testing.T) {
		rc := grown(t)
		before := rc.frame().reused
		stale := 0
		most := make([]int, len(buffers))
		for range sizeWindow + 2 {
			stale += rc.frame().stale
			for i, b := range buffers {
				most[i] = max(most[i], b.used(rc.g.recorder()))
			}
		}
		reused := rc.frame().reused
		l := rc.g.recorder()

		t.Run("and sizeWindow frames pass", func(t *testing.T) {
			for i, b := range buffers {
				t.Run("it should hold no more "+b.name+" than four times the most a frame used", func(t *testing.T) {
					assert.LessOrEqual(t, b.held(l), max(4*most[i], 2*sizeFloor))
				})
			}
			t.Run("it should keep every still drawable's record", func(t *testing.T) {
				assert.Equal(t, before, reused)
			})
			t.Run("it should hold the records made afresh", func(t *testing.T) {
				assert.Zero(t, stale)
			})
		})
	})

	t.Run("when a settled city's vertices hold far more than its frames use, and they shrink", func(t *testing.T) {
		rc := newRecordCity(t)
		for range 4 {
			rc.frame()
		}
		var freed atomic.Int32
		for range 2 {
			l := rc.g.recorder()
			room := make([]ebiten.Vertex, 0, room)
			runtime.SetFinalizer(&room[:1][0], func(*ebiten.Vertex) { freed.Add(1) })
			l.vertices = append(room, l.vertices...)
			rc.frame()
			// A frame that recorded more left a command past the end
			// of what this one recorded, still pointing into the room.
			l = rc.g.recorder()
			l.cmds = slices.Grow(l.cmds, 64)
			l.cmds[:cap(l.cmds)][cap(l.cmds)-1] = drawCmd{vs: room[:1]}
		}
		for range sizeWindow + 2 {
			rc.frame()
		}
		for range 3 {
			runtime.GC()
		}
		// The city is held past the collections, or all of it is collected.
		runtime.KeepAlive(rc)

		t.Run("it should let the room it gave back be collected", func(t *testing.T) {
			assert.Equal(t, int32(2), freed.Load())
		})
	})

	t.Run("when the city is drawn larger after its buffers shrank", func(t *testing.T) {
		rc := grown(t)
		for range sizeWindow + 2 {
			rc.frame()
		}
		require.Positive(t, rc.g.still.shrinks, "precondition: a buffer shrank")
		fit := rc.scene.Camera().Zoom
		stale := 0
		rc.scene.Camera().Zoom = 1
		for range 3 {
			stale += rc.frame().stale
		}
		rc.scene.Camera().Zoom = fit
		for range 3 {
			stale += rc.frame().stale
		}

		t.Run("it should hold the records made afresh", func(t *testing.T) {
			assert.Zero(t, stale)
		})
	})

	// frames fits a record's buffers to frames using each of uses
	// vertices in turn, and is how many times it shrank them.
	frames := func(uses ...int) int {
		g := &Game{}
		l := &drawList{vertices: make([]ebiten.Vertex, 0, room)}
		k := &keptRecord{}
		for _, n := range uses {
			k.vertices = make([]ebiten.Vertex, n)
			g.fitRecord(l, k)
		}
		return g.still.shrinks
	}
	small, big := 100, room/2
	for _, c := range []struct {
		name   string
		uses   []int
		shrank int
	}{
		{"when frames use little of a buffer for sizeWindow frames running", repeat(small, sizeWindow), 1},
		{"when they use little of it for one frame fewer", repeat(small, sizeWindow-1), 0},
		{"when frames alternate between using its room and not", alternate(big, small, 5*sizeWindow), 0},
		{"when one frame in each sizeWindow uses its room", everyNth(big, small, sizeWindow, 5*sizeWindow), 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should shrink it as often as that", func(t *testing.T) {
				assert.Equal(t, c.shrank, frames(c.uses...))
			})
		})
	}
}

func repeat(n, times int) []int {
	out := make([]int, times)
	for i := range out {
		out[i] = n
	}
	return out
}

func alternate(a, b, times int) []int {
	out := make([]int, times)
	for i := range out {
		out[i] = b
		if i%2 == 0 {
			out[i] = a
		}
	}
	return out
}

func everyNth(a, b, n, times int) []int {
	out := repeat(b, times)
	for i := 0; i < times; i += n {
		out[i] = a
	}
	return out
}
