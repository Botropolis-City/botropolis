package render

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
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

	// Item 237: a pane exactly as long as it is tall is a window, not a
	// ribbon, whichever way a camera's float noise would have tipped it.
	t.Run("when a pane exactly as long as it is tall is boarded", func(t *testing.T) {
		pane := [4]city.Point{{X: 100, Y: 200}, {X: 120, Y: 200}, {X: 120, Y: 220}, {X: 100, Y: 220}}
		left := math.Inf(1)
		for _, p := range planksFor(pane, 5) {
			for _, corner := range p {
				left = math.Min(left, corner.X)
			}
		}

		t.Run("it should cover its frame as a window's", func(t *testing.T) {
			assert.Less(t, left, 100-0.25*20)
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

// Item 237: a panning camera moves a building's foot by whole and
// fractional pixels, which must move its planks with it and change
// nothing else: no wall row, and no square pane turning ribbon.
func TestBoardsHoldStillWhileTheCameraPans(t *testing.T) {
	loaded, err := assets.LoadKits()
	require.NoError(t, err)
	k := &kits{}
	for _, a := range loaded {
		k.atlases = append(k.atlases, kitAtlas{KitAtlas: a})
	}
	// moved is every plank corner that does not simply follow the pan, as
	// the camera pans by whole and fractional pixels across both axes.
	moved := func(sprite assets.KitSprite, rows []span, scale float64, seed uint32) []string {
		start := city.Point{X: 517.3, Y: 211.9}
		still := boardsOf(sprite, rows, start, scale, seed)
		var out []string
		steps := []float64{0, 0.1, 0.2, 0.25, 0.3, 0.5, 0.7, 0.9, 1, 1.3, 1.5, 2, 2.6, 3}
		for _, across := range steps {
			for _, down := range steps {
				pan := city.Point{X: across, Y: down}
				for i, planks := range boardsOf(sprite, rows, start.Add(pan), scale, seed) {
					for p, plank := range planks {
						for c, corner := range plank {
							want := still[i][p][c].Add(pan)
							if math.Abs(corner.X-want.X) > 1e-6 || math.Abs(corner.Y-want.Y) > 1e-6 {
								out = append(out, fmt.Sprintf("pan (%g, %g): pane %d plank %d corner %d", pan.X, pan.Y, i, p, c))
							}
						}
					}
				}
			}
		}
		return out
	}
	for _, zoom := range city.ZoomSteps {
		atlas := k.atlases[k.index(zoom)]
		scale := zoom / atlas.Zoom
		for _, column := range kinds {
			for _, name := range column {
				for _, heading := range assets.Headings {
					sprite, _ := atlas.Sprite(name, heading)
					rows := wallSpans(atlas.Pages[sprite.Page].(*image.NRGBA), sprite)
					got := moved(sprite, rows, scale, 7)

					t.Run(fmt.Sprintf("when %s seen from %d at zoom %g is panned across", name, heading, zoom), func(t *testing.T) {
						t.Run("it should board every pane with the same planks, moved by the pan", func(t *testing.T) {
							assert.Empty(t, got)
						})
					})
				}
			}
		}
	}
}
