package render

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 86: planks are nailed across a shuttered building's own windows,
// covering each window's frame and sill, not only its glass.
func TestPlanks(t *testing.T) {
	for _, c := range []struct {
		face  string
		slope float64
	}{
		{"left", 0.5},
		{"right", -0.5},
	} {
		t.Run("when a pane on the "+c.face+" face is boarded", func(t *testing.T) {
			pane := [4]city.Point{
				{X: 100, Y: 200}, {X: 120, Y: 200 + 20*c.slope},
				{X: 120, Y: 230 + 20*c.slope}, {X: 100, Y: 230},
			}
			planks := planksFor(pane, 7)
			require.NotEmpty(t, planks)
			top, bottom, left, right := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
			for _, p := range planks {
				for _, corner := range p {
					top, bottom = math.Min(top, corner.Y), math.Max(bottom, corner.Y)
					left, right = math.Min(left, corner.X), math.Max(right, corner.X)
				}
			}

			t.Run("it should lay three planks across it", func(t *testing.T) {
				assert.Len(t, planks, 3)
			})

			for i, p := range planks {
				run := p[1].Sub(p[0])

				t.Run("it should run each plank near the face's slope", func(t *testing.T) {
					assert.InDelta(t, c.slope, run.Y/run.X, 0.15, "plank %d", i)
				})
			}

			t.Run("it should cover the frame on both sides of the glass", func(t *testing.T) {
				assert.Less(t, left, 100-0.25*20)
				assert.Greater(t, right, 120+0.25*20)
			})

			t.Run("it should reach above the window", func(t *testing.T) {
				assert.Less(t, top, 200-math.Abs(20*c.slope)/2)
			})

			t.Run("it should cover the sill below it", func(t *testing.T) {
				assert.Greater(t, bottom, 230+math.Abs(20*c.slope)/2+0.15*30)
			})
		})
	}

	// Items 109 and 166: the industrial kinds' ribbon windows run to the
	// edge of their face, so boards reaching past either end of the glass
	// stood out past the building's corner.
	t.Run("when a ribbon window, far wider than it is tall, is boarded", func(t *testing.T) {
		pane := [4]city.Point{{X: 100, Y: 200}, {X: 300, Y: 300}, {X: 300, Y: 306}, {X: 100, Y: 206}}
		left, right := math.Inf(1), math.Inf(-1)
		for _, p := range planksFor(pane, 5) {
			for _, corner := range p {
				left, right = math.Min(left, corner.X), math.Max(right, corner.X)
			}
		}

		t.Run("it should keep its planks within the ribbon's run", func(t *testing.T) {
			assert.True(t, left >= 100-1e-9 && right <= 300+1e-9, "planks run %.2f to %.2f", left, right)
		})
	})

	t.Run("when the same pane is boarded twice", func(t *testing.T) {
		pane := [4]city.Point{{X: 0, Y: 0}, {X: 20, Y: 10}, {X: 20, Y: 40}, {X: 0, Y: 30}}

		t.Run("it should nail the planks the same way", func(t *testing.T) {
			assert.Equal(t, planksFor(pane, 3), planksFor(pane, 3))
		})
	})
}

// Item 161: at full zoom a resting building's shutters were a mesh, a
// record and a draw for every pane, and a boarded one's two for every
// plank, its fill and its edge. A resting building's are one mesh; a
// boarded one's are one for each plank, the glass with the first, so
// that each plank still covers the nails of the planks under it.
func TestShuttersAreOneMesh(t *testing.T) {
	d := fullZoom(t, DriveOptions{})
	g := d.game
	cam := g.scene.Camera()
	var b *city.Building
	var panes int
	for _, c := range g.scene.City().Buildings() {
		if n := len(g.facadePanes(cam, buildingPiece(c), g.footOf(cam, c))); n >= 2 {
			b, panes = c, n
			break
		}
	}
	require.NotNil(t, b, "no building with two panes to shutter")
	// drawn records the building's shutters and counts its meshes and
	// its stamps, which are its nails, two a plank.
	drawn := func(boarded bool) (meshes, stamps int) {
		l := g.recorder()
		m := l.mark()
		defer l.truncate(m)
		l.begin()
		g.shutter(l.dry, cam, b, buildingPiece(b), boarded)
		in := l.end()
		for i := in.from; i < in.to; i++ {
			switch l.cmds[i].kind {
			case cmdMesh:
				meshes++
			case cmdImage:
				stamps++
			}
		}
		return meshes, stamps
	}

	t.Run("when a resting building's shutters are drawn", func(t *testing.T) {
		meshes, _ := drawn(false)

		t.Run("it should draw all its panes as one mesh", func(t *testing.T) {
			assert.Equal(t, 1, meshes, "%d panes", panes)
		})
	})

	t.Run("when a boarded building's shutters are drawn", func(t *testing.T) {
		meshes, stamps := drawn(true)
		require.Positive(t, stamps, "no plank was nailed")

		t.Run("it should draw each plank, with its edge, as one mesh", func(t *testing.T) {
			assert.Equal(t, stamps/2, meshes)
		})
	})
}
