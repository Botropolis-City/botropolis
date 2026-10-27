package acceptance_test

import (
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 308: the light is pinned at start through the light key, so the
// soak draws one light whatever the box's clock says.
func TestLightPinnedAtStart(t *testing.T) {
	helpers.NeedXvfb(t)
	home := helpers.LiveFixtureHome(t, "synthetic")
	noon := helpers.ZoneAt(12, time.Now())
	day := meanLuma(shootLit(t, home, noon, "day"))

	t.Run("when the city is shot at noon with light pinned to night", func(t *testing.T) {
		night := meanLuma(shootLit(t, home, noon, "night"))
		t.Logf("mean luma at noon: night %.2f, day %.2f", night, day)

		t.Run("it should draw it darker than day by a quarter", func(t *testing.T) {
			assert.Less(t, night, day*3/4)
		})
	})

	t.Run("when the city is shot at midnight with light live", func(t *testing.T) {
		live := meanLuma(shootLit(t, home, helpers.ZoneAt(0, time.Now()), "live"))
		t.Logf("mean luma at midnight, live: %.2f", live)

		t.Run("it should follow the clock to night", func(t *testing.T) {
			assert.Less(t, live, day*3/4)
		})
	})
}

func shootLit(t *testing.T, home, zone, light string) image.Image {
	t.Helper()
	path := filepath.Join(t.TempDir(), light+".png")
	socket := filepath.Join(t.TempDir(), "none.sock")
	cmd := exec.Command(botropolis, "city", "--headless", "--screenshot", path, "--home", home, "--socket", socket, "--set", "light="+light)
	cmd.Env = append(os.Environ(), "TZ="+zone)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	require.NoError(t, err)
	return img
}

func meanLuma(img image.Image) float64 {
	b := img.Bounds()
	var sum float64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := nrgba(img.At(x, y))
			sum += 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
		}
	}
	return sum / float64(b.Dx()*b.Dy())
}
