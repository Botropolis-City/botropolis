package render

import (
	"image/gif"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 168, slice 2: a GIF of the shared city, a few seconds long, on
// the city's own clock. Item 188: at 1080p, a frame every tick.

func TestShareGIF(t *testing.T) {
	t.Run("when sharing and g is pressed, and the city draws on", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyG, false)
		var during []string
		for i := 0; r.g.share.rec != nil && i < 1000; i++ {
			drawn := r.draw()
			if i == 10 {
				during = drawn
			}
		}
		r.g.share.encoding.Wait()
		files, err := filepath.Glob(filepath.Join(r.dir, "botropolis-share-*.gif"))
		require.NoError(t, err)
		var anim *gif.GIF
		if len(files) == 1 {
			f, err := os.Open(files[0])
			require.NoError(t, err)
			anim, err = gif.DecodeAll(f)
			_ = f.Close()
			require.NoError(t, err)
		}

		t.Run("it should say it is recording while it records", func(t *testing.T) {
			assert.True(t, slices.ContainsFunc(during, func(s string) bool { return strings.Contains(s, "recording GIF") }), "drawn: %q", during)
		})

		t.Run("it should save one GIF in the pictures folder", func(t *testing.T) {
			assert.Len(t, files, 1)
		})

		t.Run("it should hold four seconds of frames at thirty a second", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Len(t, anim.Image, gifSeconds*gifFPS)
		})

		t.Run("its frames should last four seconds between them", func(t *testing.T) {
			require.NotNil(t, anim)
			total := 0
			for _, d := range anim.Delay {
				total += d
			}
			assert.Equal(t, gifSeconds*100, total)
		})

		t.Run("each frame should last three or four hundredths of a second", func(t *testing.T) {
			require.NotNil(t, anim)
			var odd []int
			for _, d := range anim.Delay {
				if d != 3 && d != 4 {
					odd = append(odd, d)
				}
			}
			assert.Empty(t, odd)
		})

		t.Run("it should be 1920 pixels wide", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Equal(t, 1920, anim.Config.Width)
		})

		t.Run("it should be 1080 pixels high", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Equal(t, 1080, anim.Config.Height)
		})

		t.Run("it should span four seconds of the city's clock, within a tick", func(t *testing.T) {
			assert.InDelta(t, float64(gifSeconds), r.g.share.last.seconds(), 1.0/liveTPS)
		})

		t.Run("it should say where it saved", func(t *testing.T) {
			require.Len(t, files, 1)
			assert.Contains(t, r.g.notice.Text(timeNow()), files[0])
		})

		t.Run("it should still be sharing", func(t *testing.T) {
			assert.True(t, r.g.sharing())
		})
	})

	t.Run("when a GIF has finished recording", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyG, false)
		for i := 0; r.g.share.rec != nil && i < 1000; i++ {
			r.draw()
		}
		r.draw()
		r.g.share.encoding.Wait()

		t.Run("it should give the window its own size back", func(t *testing.T) {
			assert.Equal(t, city.Point{X: shareWidth, Y: shareHeight}, r.g.scene.Size())
		})
	})

	t.Run("when the window is 16:10, and a GIF starts", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.draw()
		r.press(t, ebiten.KeyG, false)
		r.draw()
		b := r.screen.Bounds()

		// The camera has one zoom for both axes, so the city could only
		// be stretched by being laid out for one size and drawn at another.
		t.Run("it should not stretch the city: it is laid out for the size it is drawn at", func(t *testing.T) {
			assert.Equal(t, city.Point{X: float64(b.Dx()), Y: float64(b.Dy())}, r.g.scene.Size())
		})
	})

	t.Run("when the view was framed by hand, and a GIF starts", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.draw()
		r.g.scene.Pan(city.Point{X: 120, Y: -40})
		cam := r.g.scene.Camera()
		centre := cam.ScreenToWorld(city.Point{X: shareWidth / 2, Y: shareHeight / 2})
		tall := shareHeight / cam.Zoom
		r.press(t, ebiten.KeyG, false)
		r.draw()
		cam = r.g.scene.Camera()
		size := r.g.scene.Size()

		t.Run("it should keep the same point at the centre", func(t *testing.T) {
			at := cam.ScreenToWorld(size.Scale(0.5))
			assert.InDelta(t, 0, math.Hypot(at.X-centre.X, at.Y-centre.Y), 1e-6)
		})

		t.Run("it should show the same height of the city", func(t *testing.T) {
			assert.InDelta(t, tall, size.Y/cam.Zoom, 1e-6)
		})
	})

	t.Run("when a GIF is recording and esc is pressed", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyG, false)
		for range 5 {
			r.draw()
		}
		r.press(t, ebiten.KeyEscape, false)
		r.g.ticks++
		r.g.Draw(r.screen)
		files, err := filepath.Glob(filepath.Join(r.dir, "botropolis-share-*"))
		require.NoError(t, err)

		t.Run("it should stop sharing", func(t *testing.T) {
			assert.False(t, r.g.sharing())
		})

		t.Run("it should save nothing", func(t *testing.T) {
			assert.Empty(t, files)
		})
	})

	t.Run("when the window closes", func(t *testing.T) {
		gifs := func(t *testing.T, r *shareRig) []string {
			t.Helper()
			files, err := filepath.Glob(filepath.Join(r.dir, "botropolis-share-*.gif"))
			require.NoError(t, err)
			return files
		}

		t.Run("and a GIF has its last frame but is still being written", func(t *testing.T) {
			r := newShareRig(t)
			r.press(t, ebiten.KeyS, true)
			r.press(t, ebiten.KeyG, false)
			for i := 0; r.g.share.rec != nil && i < 1000; i++ {
				r.draw()
			}
			r.g.finishSharing()

			t.Run("it should wait for the file", func(t *testing.T) {
				assert.Len(t, gifs(t, r), 1)
			})
		})

		t.Run("and a GIF is halfway through recording", func(t *testing.T) {
			r := newShareRig(t)
			r.press(t, ebiten.KeyS, true)
			r.press(t, ebiten.KeyG, false)
			for range 20 {
				r.draw()
			}
			r.g.finishSharing()

			t.Run("it should write nothing", func(t *testing.T) {
				assert.Empty(t, gifs(t, r))
			})
		})
	})

	t.Run("when a GIF is recording", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyG, false)

		t.Run("it should hold the city's clock to one update a drawn frame", func(t *testing.T) {
			r.g.painted = r.g.ticks - 1
			assert.False(t, r.g.startTick())
		})
	})
}
