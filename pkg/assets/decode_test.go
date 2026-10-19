package assets_test

import (
	"image"
	"image/color"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
)

// Item 70: a page decoded on its own is the page LoadKits decodes, so
// the window can decode and upload one page at a time.
func TestDecodePage(t *testing.T) {
	decoded, err := assets.LoadKits()
	require.NoError(t, err)
	manifests, err := assets.LoadKitManifests()
	require.NoError(t, err)
	require.Len(t, manifests, len(decoded))

	t.Run("when the manifests are read", func(t *testing.T) {
		t.Run("it should decode no page", func(t *testing.T) {
			for _, a := range manifests {
				assert.Empty(t, a.Pages, "zoom %g", a.Zoom)
			}
		})

		t.Run("it should count every page LoadKits decodes", func(t *testing.T) {
			for i, a := range manifests {
				assert.Equal(t, len(decoded[i].Pages), a.PageCount(), "zoom %g", a.Zoom)
			}
		})
	})

	t.Run("when one page is decoded on its own", func(t *testing.T) {
		last := manifests[len(manifests)-1]
		want := decoded[len(decoded)-1].Pages[last.PageCount()-1]
		got, err := last.DecodePage(last.PageCount() - 1)
		require.NoError(t, err)

		t.Run("it should match the page LoadKits decodes, pixel for pixel", func(t *testing.T) {
			assert.True(t, samePixels(want, got))
		})

		t.Run("and a retint was asked for first", func(t *testing.T) {
			const stack = "city-kit-industrial/chimney-medium"
			target := color.NRGBA{0x6a, 0x8c, 0xb8, 0xff}
			fresh, err := assets.LoadKits()
			require.NoError(t, err)
			whole := fresh[0]
			whole.RetintAmber([]string{stack}, target)
			lazy, err := assets.LoadKitManifests()
			require.NoError(t, err)
			one := lazy[0]
			one.RetintAmber([]string{stack}, target)
			s, ok := one.Sprite(stack, 0)
			require.True(t, ok)
			page, err := one.DecodePage(s.Page)
			require.NoError(t, err)

			t.Run("it should match LoadKits' page after the same retint", func(t *testing.T) {
				assert.True(t, samePixels(whole.Pages[s.Page], page))
			})
		})
	})

	t.Run("when a page past the last is asked for", func(t *testing.T) {
		a := manifests[0]
		_, err := a.DecodePage(a.PageCount())

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when every page has been decoded in turn and dropped", func(t *testing.T) {
		lazy, err := assets.LoadKitManifests()
		require.NoError(t, err)
		before := liveHeap()
		for i := range lazy {
			for p := range lazy[i].PageCount() {
				_, err := lazy[i].DecodePage(p)
				require.NoError(t, err)
			}
		}
		grew := int64(liveHeap()) - int64(before)
		onePage := int64(4 * 2048 * 2048)

		t.Run("it should hold less than one page of heap more than before", func(t *testing.T) {
			assert.Less(t, grew, onePage)
		})
		runtime.KeepAlive(lazy)
	})
}

func liveHeap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func samePixels(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if color.NRGBAModel.Convert(a.At(x, y)) != color.NRGBAModel.Convert(b.At(x, y)) {
				return false
			}
		}
	}
	return true
}
