package render

import "github.com/hajimehoshi/ebiten/v2"

// roomForMesh makes room in the list's buffers for a mesh of nv vertices
// and ni indices. A recorded mesh's commands hold slices of the buffers,
// so a buffer grown by append left every command recorded before it
// holding the old one: a first frame of 600 districts held 330 MB of
// dead buffers, over the city's memory limit (item 315). A buffer grown
// here doubles, and every unowned mesh is moved onto it.
func (l *drawList) roomForMesh(nv, ni int) {
	moved := false
	if need := len(l.vertices) + nv; need > cap(l.vertices) {
		l.vertices, moved = regrown(l.vertices, need), true
	}
	if need := len(l.indices) + ni; need > cap(l.indices) {
		l.indices, moved = regrown(l.indices, need), true
	}
	if !moved {
		return
	}
	for i := range l.cmds {
		c := &l.cmds[i]
		if c.kind != cmdMesh || c.owned {
			continue
		}
		v, n := int(c.at[0]), int(c.at[1])
		c.vs = l.vertices[v : v+len(c.vs) : v+len(c.vs)]
		c.is = l.indices[n : n+len(c.is) : n+len(c.is)]
	}
}

// meshBufferMin is the least a mesh buffer grows to.
const meshBufferMin = 1024

// regrown is buf copied into a buffer of at least need, twice as big as
// it was.
func regrown[T ebiten.Vertex | uint32](buf []T, need int) []T {
	grown := make([]T, len(buf), max(2*cap(buf), need, meshBufferMin))
	copy(grown, buf)
	return grown
}
