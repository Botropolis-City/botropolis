package acceptance_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
	"botropolis.city/botropolis/testing/helpers"
)

// shoot renders one frame of the sample city at a scale with no daemon
// to talk to, so the client scans the fixture home directly.
func shoot(t *testing.T, home, scale string) image.Image {
	t.Helper()
	path := filepath.Join(t.TempDir(), "city-"+scale+"x.png")
	socket := filepath.Join(t.TempDir(), "none.sock")
	// n twice forces day, so the ground is the daytime green whatever the
	// fixture's sessions are doing.
	out, err := exec.Command(botropolis, "city", "--headless", "--screenshot", path, "--render_scale", scale, "--home", home, "--socket", socket, "--keys", "n,n").CombinedOutput()
	require.NoError(t, err, string(out))
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	require.NoError(t, err)
	return img
}

func nrgba(c color.Color) color.NRGBA {
	r, g, b, a := c.RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func isTone(c color.NRGBA) bool {
	p := ui.DefaultPalette
	for _, tone := range []color.NRGBA{p.NeedsYou, p.Working, p.Unattended, p.Parked} {
		if c == tone {
			return true
		}
	}
	return false
}

// darkBand is how many rows down from the top the strip's dark panel
// runs at the left edge.
func darkBand(img image.Image) int {
	h := img.Bounds().Dy()
	for y := 0; y < h; y++ {
		if !isDark(nrgba(img.At(2, y))) {
			return y
		}
	}
	return h
}

func isDark(c color.NRGBA) bool {
	return c.R < 0x40 && c.G < 0x40 && c.B < 0x40
}

func isGround(c color.NRGBA) bool {
	return c == ui.DefaultPalette.Ground
}

func TestCityScreenshot(t *testing.T) {
	if _, err := exec.LookPath("xvfb-run"); err != nil {
		t.Skip("no xvfb-run to open the city on a virtual display")
	}
	home := helpers.LiveFixtureHome(t, "sample")

	t.Run("when the city is shot at 1x and 2x", func(t *testing.T) {
		one := shoot(t, home, "1")
		two := shoot(t, home, "2")
		w, h := one.Bounds().Dx(), one.Bounds().Dy()

		t.Run("it should draw the strip twice as tall at 2x", func(t *testing.T) {
			// The window manager may tile the two windows to any size, so
			// the frames are compared by the strip's height, which only
			// the scale sets.
			assert.InDelta(t, 2*darkBand(one), darkBand(two), 2)
		})

		t.Run("it should draw the strip's first dot in a state tone", func(t *testing.T) {
			assert.True(t, isTone(nrgba(one.At(20, 16))), nrgba(one.At(20, 16)))
		})

		t.Run("it should draw the same dot at twice the offset at 2x", func(t *testing.T) {
			assert.Equal(t, nrgba(one.At(20, 16)), nrgba(two.At(40, 32)))
		})

		t.Run("it should draw the strip on a dark panel", func(t *testing.T) {
			assert.True(t, isDark(nrgba(one.At(2, 2))), nrgba(one.At(2, 2)))
		})

		t.Run("it should draw the footer on a dark panel", func(t *testing.T) {
			assert.True(t, isDark(nrgba(one.At(2, h-2))), nrgba(one.At(2, h-2)))
		})

		t.Run("it should show the ground somewhere between them", func(t *testing.T) {
			found := false
			for y := 40; y < h-40 && !found; y++ {
				for x := w / 4; x < 3*w/4; x += 8 {
					if isGround(nrgba(one.At(x, y))) {
						found = true
						break
					}
				}
			}
			assert.True(t, found)
		})
	})
}
