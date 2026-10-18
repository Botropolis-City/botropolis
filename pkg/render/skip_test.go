package render

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 109: how full a live session's context is shows as a skip at the
// kerb beside its building.
func TestSkipFill(t *testing.T) {
	cases := []struct {
		percent float64
		want    string
	}{
		{49, ""}, {50, "botropolis/skip-1"}, {64, "botropolis/skip-1"}, {65, "botropolis/skip-2"},
		{80, "botropolis/skip-3"}, {94, "botropolis/skip-3"}, {95, "botropolis/skip-4"}, {100, "botropolis/skip-4"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("when a live session is at %g%%", c.percent), func(t *testing.T) {
			b := &city.Building{Session: state.Session{State: state.Working, ContextPercent: c.percent}}

			t.Run(fmt.Sprintf("it should show %q", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, skipPiece(shownSkip(b, city.Job{}, false)))
			})
		})
	}

	t.Run("when the session is stopped", func(t *testing.T) {
		t.Run("and its context was at 90%", func(t *testing.T) {
			b := &city.Building{Session: state.Session{State: state.Resting, ContextPercent: 90}}

			t.Run("it should show no skip", func(t *testing.T) {
				assert.Empty(t, skipPiece(shownSkip(b, city.Job{}, false)))
			})
		})
	})
}

func TestSkipPlacement(t *testing.T) {
	g := kitGame(t)
	cam := city.NewCamera()
	for _, kind := range city.DistrictTypes {
		for n := 1; n <= state.MaxStoreys; n++ {
			b := kindBuilding(kind, n)
			sprite, ok := g.kits.pick(cam.Zoom).Sprite(buildingPiece(b), cam.Heading)
			require.True(t, ok)
			at := g.skipAt(cam, b)
			t.Run(fmt.Sprintf("when a %s building of %d storeys has a skip", kind, n), func(t *testing.T) {
				t.Run("it should stand on the building's lot", func(t *testing.T) {
					assert.True(t, b.Rect.Contains(at), "%v not in %v", at, b.Rect)
				})

				t.Run("it should stand outside the building's footprint", func(t *testing.T) {
					c := b.Rect.Center()
					c.X += sprite.Body.X * city.CellSize
					body := city.RectAt(c.X-sprite.Body.W*city.CellSize/2, c.Y-sprite.Body.D*city.CellSize/2, sprite.Body.W*city.CellSize, sprite.Body.D*city.CellSize)
					assert.False(t, body.Contains(at), "%v inside %v", at, body)
				})
			})
		}
	}
}

func TestSkipDuringCollection(t *testing.T) {
	b := &city.Building{Session: state.Session{State: state.Working, ContextPercent: 12}}

	t.Run("when a job has started and the skip truck has not hooked it yet", func(t *testing.T) {
		job := city.Job{SkipLevel: 3, SkipAtKerb: true}

		t.Run("it should show the skip as it was before the compaction", func(t *testing.T) {
			assert.Equal(t, "botropolis/skip-3", skipPiece(shownSkip(b, job, true)))
		})
	})

	t.Run("and after the hook", func(t *testing.T) {
		job := city.Job{SkipLevel: 3}

		t.Run("it should show no skip at the kerb", func(t *testing.T) {
			assert.Empty(t, skipPiece(shownSkip(b, job, true)))
		})
	})
}

func TestJobPieces(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)

	t.Run("when the compaction job's pieces are looked up in the atlas", func(t *testing.T) {
		var missing []string
		for _, name := range []string{skipPiece(1), skipPiece(2), skipPiece(3), skipPiece(4), kitSkipTruck, kitSkipTruckLoaded, kitMixer} {
			if _, ok := atlases[0].Sprite(name, 0); !ok {
				missing = append(missing, name)
			}
		}

		t.Run("it should have the skips, the skip truck loaded and empty, and the mixer", func(t *testing.T) {
			assert.Empty(t, missing)
		})
	})
}
