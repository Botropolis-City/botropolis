package render

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 233: a look is baked where the building stands on the frame, with
// the frame's own camera, into the scratch a pan shifts layers through,
// and copied from there onto its page by whole pixels. Baked with the
// camera moved, every screen value carried float noise of its own, and
// whatever sat exactly on a tie (a wall row, a square pane, a glyph's
// edge on a pixel centre) came out the other way in the look.
func TestBakeDrawsAtTheFramesCoordinates(t *testing.T) {
	frame := image.Rect(0, 0, 512, 512)

	t.Run("when a run's look box lies inside the frame", func(t *testing.T) {
		region, ok := bakeRegion(city.Rect{Min: city.Point{X: 100.3, Y: 50.7}, Max: city.Point{X: 180.2, Y: 140.1}}, frame)

		t.Run("it should bake it", func(t *testing.T) {
			assert.True(t, ok)
		})

		t.Run("it should bake it from the look's box in frame coordinates, padded", func(t *testing.T) {
			assert.Equal(t, image.Rect(100-lookPad, 50-lookPad, 181+lookPad, 141+lookPad), region)
		})
	})

	for _, c := range []struct {
		side string
		box  city.Rect
	}{
		{"left", city.Rect{Min: city.Point{X: 2, Y: 50}, Max: city.Point{X: 80, Y: 140}}},
		{"top", city.Rect{Min: city.Point{X: 100, Y: 1}, Max: city.Point{X: 180, Y: 140}}},
		{"right", city.Rect{Min: city.Point{X: 450, Y: 50}, Max: city.Point{X: 510, Y: 140}}},
		{"bottom", city.Rect{Min: city.Point{X: 100, Y: 450}, Max: city.Point{X: 180, Y: 509.5}}},
	} {
		t.Run("when the look's box runs off the frame's "+c.side, func(t *testing.T) {
			_, ok := bakeRegion(c.box, frame)

			t.Run("it should not be baked", func(t *testing.T) {
				assert.False(t, ok)
			})
		})
	}

	t.Run("when a run is baked", func(t *testing.T) {
		g := bakeGame(t)
		scratch := ebiten.NewImage(lookCellSize, lookCellSize)
		g.still.scratch = scratch
		stats := g.drawSteadily(lookFoot)
		require.NotZero(t, stats.bakes, "precondition: a look was baked")

		t.Run("it should allocate no texture beyond the shared scratch", func(t *testing.T) {
			assert.Same(t, scratch, g.still.scratch)
		})
	})

	t.Run("when a run is baked with no scratch held", func(t *testing.T) {
		g := bakeGame(t)
		stats := g.drawSteadily(lookFoot)
		require.NotZero(t, stats.bakes, "precondition: a look was baked")

		t.Run("it should hold the shared scratch", func(t *testing.T) {
			assert.NotNil(t, g.still.scratch)
		})
	})

	t.Run("when the building stands so its look runs off the frame", func(t *testing.T) {
		g := bakeGame(t)
		edge := city.Point{X: 3.3, Y: lookFoot.Y}
		stats := g.drawSteadily(edge)

		t.Run("it should bake its look", func(t *testing.T) {
			assert.NotZero(t, stats.bakes)
		})

		t.Run("it should mark the look inexact", func(t *testing.T) {
			assert.True(t, g.onlyLook(t).inexact)
		})

		t.Run("and it is then drawn wholly on the frame", func(t *testing.T) {
			on := g.drawAt(city.Point{X: edge.X + 250, Y: edge.Y})

			t.Run("it should bake the look again", func(t *testing.T) {
				assert.Equal(t, 1, on.bakes)
			})

			t.Run("it should bake it exactly", func(t *testing.T) {
				assert.False(t, g.onlyLook(t).inexact)
			})

			t.Run("and it is drawn there again", func(t *testing.T) {
				again := g.drawAt(city.Point{X: edge.X + 250, Y: edge.Y})

				t.Run("it should draw it from the cache without baking it", func(t *testing.T) {
					assert.Equal(t, lookStats{hits: again.runs, runs: again.runs}, again)
				})
			})
		})
	})

	t.Run("when a look has been baked through the scratch", func(t *testing.T) {
		t.Run("and nothing uses the scratch for scratchHold frames", func(t *testing.T) {
			g := &Game{}
			g.lookScratch(frame)
			for range scratchHold {
				g.restScratch()
			}

			t.Run("it should let the scratch go", func(t *testing.T) {
				assert.Nil(t, g.still.scratch)
			})
		})

		t.Run("and another is baked within scratchHold frames", func(t *testing.T) {
			g := &Game{}
			g.lookScratch(frame)
			for i := range scratchHold {
				g.restScratch()
				if i == scratchHold/2 {
					g.lookScratch(frame)
				}
			}

			t.Run("it should keep the scratch", func(t *testing.T) {
				assert.NotNil(t, g.still.scratch)
			})
		})
	})
}

// bakeGame is a city of kits and faces with nothing in it, as the sheet
// draws its buildings.
func bakeGame(t *testing.T) *Game {
	t.Helper()
	k, err := loadKits()
	require.NoError(t, err)
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	require.NoError(t, err)
	return &Game{kits: k, faces: f, theme: th, scene: city.NewScene(nil)}
}

// drawSteadily draws a dressed tower through the look cache at zoom 1.5
// with its foot at a point on a frame, for as many frames as its look
// takes to be baked, and says what the cache did on the last.
func (g *Game) drawSteadily(foot city.Point) lookStats {
	g.drawAt(foot)
	return g.drawAt(foot)
}

// drawAt draws one frame of a dressed tower through the look cache at
// zoom 1.5 with its foot at a point on a frame, and says what the cache
// did.
func (g *Game) drawAt(foot city.Point) lookStats {
	b := dressedBuilding()
	cam := &city.Camera{Zoom: 1.5}
	at := cam.Project(b.Rect.Center())
	cam.Offset = city.Point{X: foot.X/cam.Zoom - at.X, Y: foot.Y/cam.Zoom - at.Y}
	img := ebiten.NewImage(lookCellSize, lookCellSize)
	g.nextLookFrame()
	g.drawBuilding(img, cam, b, false, true, 0)
	return g.lookStats
}

// onlyLook is the one look the cache holds.
func (g *Game) onlyLook(t *testing.T) *bakedLook {
	t.Helper()
	var looks []*bakedLook
	for _, e := range g.looks.entries {
		looks = append(looks, e.Value.(*lookEntry[lookKey, *bakedLook]).value)
	}
	require.Len(t, looks, 1, "precondition: the cache holds one look")
	return looks[0]
}
