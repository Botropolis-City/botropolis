package render

import (
	"math"
	"testing"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zoomPicking is the least camera zoom that draws with level i. Since item
// 162's slice 14 that is not the level's own cut: z1.5 is picked only from
// 1.75, so a test that set the camera to atlas.Zoom drew with z1 twice.
func zoomPicking(t testing.TB, k *kits, i int) float64 {
	t.Helper()
	zoom := k.atlases[i].Zoom
	if i > 0 {
		zoom = max(zoom, k.atlases[i-1].Zoom*assets.KitDrawUpMost)
	}
	require.Equal(t, i, k.index(zoom), "precondition: zoom %g picks level %d", zoom, i)
	return zoom
}

// Bug 53. Aria: "The boats get smaller at a certain zoom. We don't need
// to do that."
//
// kitSized states a size and works the scale back from the art, which
// is what item 51 needed. It did that in SCREEN pixels, so the sprite's
// own size cancelled out of the arithmetic and the drawn size came to
// depend only on cam.Zoom/atlas.Zoom. atlas.Zoom is a step function, so
// at the point where the z2 cut takes over, the ratio halves and the
// boat halves with it. No threshold was ever written; the boundary is
// the whole of the bug.
//
// Every other piece survives that step because the ladder's
// normalisation is the point of the ladder: a z2 sprite is about twice
// its z1 cut, which cancels the halved ratio. kitSized opted out of
// that, and this is the guard that it has opted back in.

func testKits(t *testing.T) *kits {
	t.Helper()
	loaded, err := assets.LoadKits()
	require.NoError(t, err)
	k := &kits{}
	for _, a := range loaded {
		k.atlases = append(k.atlases, kitAtlas{KitAtlas: a})
	}
	require.Greater(t, len(k.atlases), 1, "the step needs two atlases to exist")
	return k
}

// drawnLongest is the longest side a piece comes out at on screen when
// drawn through kitSized, in the atlas given. It is the renderer's own
// expression — kitShrink — rather than a copy of it.
func drawnLongest(t *testing.T, atlas *kitAtlas, piece string, target, camZoom float64) float64 {
	t.Helper()
	sprite, ok := atlas.Sprite(piece, 0)
	require.True(t, ok, piece)
	longest := math.Max(float64(sprite.Rect.Dx()), float64(sprite.Rect.Dy()))
	return longest * camZoom / atlas.Zoom * kitShrink(target, longest, atlas.Zoom)
}

func TestKitSizedIgnoresWhichAtlasItIsCutFrom(t *testing.T) {
	k := testKits(t)
	one, two := &k.atlases[0], &k.atlases[len(k.atlases)-1]
	require.NotEqual(t, one.Zoom, two.Zoom)

	for _, piece := range []string{kitGaugeLiner, kitGaugeCargo, kitGaugeSail, kitBuoy} {
		t.Run("when "+piece+" is drawn at one zoom from either cut", func(t *testing.T) {
			const camZoom = 2.0
			from1 := drawnLongest(t, one, piece, 142, camZoom)
			from2 := drawnLongest(t, two, piece, 142, camZoom)

			t.Run("it should come out the same size, whichever the ladder picked", func(t *testing.T) {
				assert.InDelta(t, 1.0, from2/from1, 0.02,
					"z%g draws %.1f px and z%g draws %.1f", one.Zoom, from1, two.Zoom, from2)
			})
		})
	}
}

func TestKitSizedGrowsAllTheWayUpTheLadder(t *testing.T) {
	k := testKits(t)

	t.Run("when the camera climbs every step of the zoom ladder", func(t *testing.T) {
		// End to end through the real pick, so the atlas the ladder
		// actually chooses is the one measured. Aria's "certain zoom" is
		// the last step, where the second cut takes over.
		var sizes []float64
		for _, zoom := range city.ZoomSteps {
			sizes = append(sizes, drawnLongest(t, k.pick(zoom), kitGaugeLiner, 142, zoom))
		}

		t.Run("it should never draw the boat smaller than at the step below", func(t *testing.T) {
			for i := 1; i < len(sizes); i++ {
				require.Greater(t, sizes[i], sizes[i-1],
					"zoom %g draws %.1f px, down from %.1f at %g",
					city.ZoomSteps[i], sizes[i], sizes[i-1], city.ZoomSteps[i-1])
			}
			assert.Len(t, sizes, len(city.ZoomSteps))
		})
	})
}
