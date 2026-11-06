package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 348, slice 2: the ground layer is composed again when the kerbs
// change, since they are painted into it.
func TestKerbsKey(t *testing.T) {
	t.Run("when the kerbs are turned off", func(t *testing.T) {
		rc := newRecordCity(t)
		cam, hover := rc.scene.Camera(), rc.scene.Hover()
		before := rc.g.keyFor(cam, hover, recordWidth, recordHeight)
		rc.scene.SetKerbs(city.KerbsOff)
		after := rc.g.keyFor(cam, hover, recordWidth, recordHeight)

		t.Run("it should change the ground layer's key", func(t *testing.T) {
			assert.NotEqual(t, before, after)
		})
	})
}

// Item 348, slice 2: the kerb is a low wall raised along the outline,
// so it reads past the buildings beside it (Aria, 2026-11-06).
func TestKerbWall(t *testing.T) {
	cam := city.NewCamera()
	cam.Projection = city.Isometric
	cam.Zoom = 2
	edge := [2]city.Point{{X: 0, Y: 0}, {X: 32, Y: 0}}
	wall := kerbWall(cam, edge)
	a, b := cam.WorldToScreen(edge[0]), cam.WorldToScreen(edge[1])

	t.Run("when an edge of the outline is raised", func(t *testing.T) {
		t.Run("it should stand on the edge", func(t *testing.T) {
			assert.Equal(t, [2]city.Point{a, b}, [2]city.Point{wall[0], wall[1]})
		})

		t.Run("it should rise kerbRise pixels a zoom above it", func(t *testing.T) {
			assert.Equal(t, [2]city.Point{{X: b.X, Y: b.Y - kerbRise*2}, {X: a.X, Y: a.Y - kerbRise*2}}, [2]city.Point{wall[2], wall[3]})
		})
	})
}
