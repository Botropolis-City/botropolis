package render

import (
	"slices"
	"unsafe"
)

// The light-cycle check reads the live heap less the slack the record's
// two halves hold, which the rule in recordsize.go permits and which
// moves from run to run with no growth behind it (item 318). So that
// the subtraction cannot hide a rule that stopped cutting back, each
// half is held to it every frame the log is asked for: no buffer holds
// more than four times the most it used over the last sizeWindow
// frames, or the floor's room, for two frames running, the first being
// the grace a half has just after the swap before it is fitted.

// recordBufferNames name the record's buffers in the order fitRecord
// sizes them.
var recordBufferNames = [10]string{"cmds", "vertices", "indices", "points", "imgOps", "cmOps", "triOps", "pathOps", "textOps", "fills"}

// bufferRoom is one of the record's buffers as the rule sees it.
type bufferRoom struct{ len, cap, size int }

func roomOf[T any](s []T) bufferRoom {
	var z T
	return bufferRoom{len(s), cap(s), int(unsafe.Sizeof(z))}
}

// recordRule is what the rule saw the last frame it was noted.
type recordRule struct {
	over   [2][10]bool
	slack  int64
	broken string
}

// recordHalves are the record's two halves, live and kept, each buffer
// in fitRecord's order.
func (g *Game) recordHalves() [2][10]bufferRoom {
	l, k := g.recorder(), &g.still.kept
	return [2][10]bufferRoom{
		{roomOf(l.cmds), roomOf(l.vertices), roomOf(l.indices), roomOf(l.points), roomOf(l.imgOps),
			roomOf(l.cmOps), roomOf(l.triOps), roomOf(l.pathOps), roomOf(l.textOps), roomOf(l.fills)},
		{roomOf(k.cmds), roomOf(k.vertices), roomOf(k.indices), roomOf(k.points), roomOf(k.imgOps),
			roomOf(k.cmOps), roomOf(k.triOps), roomOf(k.pathOps), roomOf(k.textOps), roomOf(k.fills)},
	}
}

// noteRecordRule notes the record's slack this frame and holds each half
// to the rule, naming the first half and buffer found outside it two
// frames running.
func (g *Game) noteRecordRule() {
	halves := g.recordHalves()
	r := &g.recordRule
	r.slack = 0
	for i := range recordBufferNames {
		peak := max(slices.Max(g.still.sizes[i].used[:]), halves[0][i].len, halves[1][i].len)
		allowed := max(4*peak, 2*sizeFloor)
		for h, half := range halves {
			b := half[i]
			r.slack += int64((b.cap - b.len) * b.size)
			over := b.cap > allowed
			if over && r.over[h][i] && r.broken == "" {
				r.broken = [2]string{"live", "kept"}[h] + "/" + recordBufferNames[i]
			}
			r.over[h][i] = over
		}
	}
}

// recordReading is the record's slack as last noted, in bytes, and the
// half and buffer that broke the rule, if any has.
func (g *Game) recordReading() (int64, string) {
	return g.recordRule.slack, g.recordRule.broken
}
