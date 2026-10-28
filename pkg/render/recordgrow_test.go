package render

import (
	"image"
	"testing"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/shapes"
)

// inBuffer is whether s lies inside buf's backing array.
func inBuffer[T any](s, buf []T) bool {
	if len(s) == 0 {
		return true
	}
	size := unsafe.Sizeof(buf[:1][0])
	lo := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	hi := lo + uintptr(cap(buf))*size
	at := uintptr(unsafe.Pointer(unsafe.SliceData(s)))
	return at >= lo && at+uintptr(len(s))*size <= hi
}

// meshVertices is a mesh of n vertices, each marked with seed and its
// place, so one mesh's vertices are told from another's.
func meshVertices(seed, n int) ([]ebiten.Vertex, []uint32) {
	vs := make([]ebiten.Vertex, n)
	is := make([]uint32, n)
	for i := range vs {
		vs[i] = ebiten.Vertex{DstX: float32(seed), DstY: float32(i)}
		is[i] = uint32(i)
	}
	return vs, is
}

// meshesOut is whether every unowned mesh command in l lies in l's own
// buffers, and whether each still holds the vertices it was given.
func meshesOut(l *drawList, sizes []int) (vertsIn, indicesIn, same bool) {
	vertsIn, indicesIn, same = true, true, true
	m := 0
	for i := range l.cmds {
		c := &l.cmds[i]
		if c.kind != cmdMesh {
			continue
		}
		vertsIn = vertsIn && inBuffer(c.vs, l.vertices)
		indicesIn = indicesIn && inBuffer(c.is, l.indices)
		want, _ := meshVertices(m, sizes[m])
		if len(c.vs) != len(want) {
			same = false
		} else {
			for j := range want {
				same = same && c.vs[j].DstX == want[j].DstX && c.vs[j].DstY == want[j].DstY
			}
		}
		m++
	}
	return vertsIn, indicesIn, same
}

// Item 315: each mesh command kept slices of the list's buffers, so
// every time a buffer grew, the commands already recorded held the old
// one: a first frame of 600 districts held 330 MB of dead buffers, and
// the collector ran flat out at the city's memory limit.
func TestDrawListMeshBuffers(t *testing.T) {
	sizes := make([]int, 200)
	for i := range sizes {
		sizes[i] = 3 + i%50
	}
	src := ebiten.NewImage(1, 1)
	record := func(l *drawList) {
		l.begin()
		for m, n := range sizes {
			vs, is := meshVertices(m, n)
			l.Mesh(l.dry, vs, is, src, &ebiten.DrawTrianglesOptions{}, shapes.MeshKey{})
		}
		l.end()
	}

	t.Run("when a list records more meshes than its buffers hold", func(t *testing.T) {
		l := newDrawList()
		record(l)
		vertsIn, indicesIn, same := meshesOut(l, sizes)

		t.Run("it should hold every recorded mesh's vertices in its own vertex buffer", func(t *testing.T) {
			assert.True(t, vertsIn)
		})

		t.Run("it should hold every recorded mesh's indices in its own index buffer", func(t *testing.T) {
			assert.True(t, indicesIn)
		})

		t.Run("it should play back the vertices each mesh was recorded with", func(t *testing.T) {
			assert.True(t, same)
		})
	})

	t.Run("when kept meshes are copied onto a record whose buffers are full", func(t *testing.T) {
		kept := newDrawList()
		record(kept)
		k := &keptRecord{cmds: kept.cmds, vertices: kept.vertices, indices: kept.indices, triOps: kept.triOps}
		l := newDrawList()
		l.vertices = make([]ebiten.Vertex, 0, 1)
		l.indices = make([]uint32, 0, 1)
		g := &Game{rec: l}
		g.copyCmds(k, 0, len(k.cmds), image.Point{})
		vertsIn, indicesIn, same := meshesOut(l, sizes)

		t.Run("it should hold every copied mesh's vertices in its own vertex buffer", func(t *testing.T) {
			assert.True(t, vertsIn)
		})

		t.Run("it should hold every copied mesh's indices in its own index buffer", func(t *testing.T) {
			assert.True(t, indicesIn)
		})

		t.Run("it should play back the vertices each copied mesh was recorded with", func(t *testing.T) {
			assert.True(t, same)
		})
	})
}
