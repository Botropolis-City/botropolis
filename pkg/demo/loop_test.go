package demo

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeGrey(t *testing.T, path string, level uint8) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: level, G: level, B: level, A: 255})
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	require.NoError(t, png.Encode(f, img))
}

func readGrey(t *testing.T, path string) float64 {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	require.NoError(t, err)
	r, _, _, _ := img.At(0, 0).RGBA()
	return float64(r >> 8)
}

// Item 76: the hero is a loop, so its last frame has to lead into its
// first. Each recorded frame's grey is its index, so every output frame
// says which recorded frames it was made from.
func TestLoopFrames(t *testing.T) {
	const lead, n, c = 3, 20, 4
	recorded := t.TempDir()
	for i := range lead + n + c {
		writeGrey(t, filepath.Join(recorded, fmt.Sprintf("frame-%05d.png", i)), uint8(i*8))
	}
	out := t.TempDir()
	require.NoError(t, loopFrames(recorded, out, lead, n, c))
	grey := func(i int) float64 { return readGrey(t, filepath.Join(out, fmt.Sprintf("frame-%05d.png", i))) }

	t.Run("when a recording loops with a lead, a clip and a crossfade", func(t *testing.T) {
		t.Run("it should write the clip's frames", func(t *testing.T) {
			written, err := filepath.Glob(filepath.Join(out, "frame-*.png"))
			require.NoError(t, err)
			assert.Len(t, written, n)
		})

		t.Run("it should open on the recorded frame after the lead and the crossfade", func(t *testing.T) {
			assert.Equal(t, float64((lead+c)*8), grey(0))
		})

		t.Run("it should play the recording straight up to the crossfade", func(t *testing.T) {
			var got, want []float64
			for i := range n - c {
				got, want = append(got, grey(i)), append(want, float64((lead+c+i)*8))
			}
			assert.Equal(t, want, got)
		})

		for j := range c {
			t.Run(fmt.Sprintf("it should blend crossfade frame %d toward recorded frame lead+%d by %d/%d", j, j, j+1, c+1), func(t *testing.T) {
				i := n - c + j
				w := float64(j+1) / float64(c+1)
				want := (1-w)*float64((lead+c+i)*8) + w*float64((lead+j)*8)
				assert.InDelta(t, want, grey(i), 1)
			})
		}
	})

	t.Run("when the recording is short of frames", func(t *testing.T) {
		err := loopFrames(recorded, t.TempDir(), lead, n+1, c)

		t.Run("it should refuse rather than loop a shorter clip", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
