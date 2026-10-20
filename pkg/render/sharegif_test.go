package render

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 168, slice 2: a GIF of the shared city, a few seconds long and
// sized for a chat or an issue, on the city's own clock.

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

		t.Run("it should hold four seconds of frames at fifteen a second", func(t *testing.T) {
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

		t.Run("it should be at most 720 pixels wide", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Equal(t, gifMaxWidth, anim.Config.Width)
		})

		t.Run("it should keep the window's shape", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Equal(t, gifMaxWidth*shareHeight/shareWidth, anim.Config.Height)
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

func TestQuantize(t *testing.T) {
	t.Run("when a frame has fewer than 256 colours", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 40, 30))
		colours := []color.RGBA{{0x10, 0x20, 0x30, 0xff}, {0xf0, 0xb4, 0x4c, 0xff}, {0x5c, 0xc8, 0xb0, 0xff}}
		for y := range 30 {
			for x := range 40 {
				img.SetRGBA(x, y, colours[(x+y)%len(colours)])
			}
		}
		p := quantize(img)
		var changed int
		for y := range 30 {
			for x := range 40 {
				if color.RGBAModel.Convert(p.At(x, y)) != img.At(x, y) {
					changed++
				}
			}
		}

		t.Run("it should keep every pixel's colour", func(t *testing.T) {
			assert.Zero(t, changed)
		})
	})

	t.Run("when a frame has thousands of colours", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 256, 64))
		for y := range 64 {
			for x := range 256 {
				img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y * 4), uint8(255 - x), 0xff})
			}
		}
		p := quantize(img)
		var worst int
		for y := range 64 {
			for x := range 256 {
				a := color.RGBAModel.Convert(p.At(x, y)).(color.RGBA)
				b := img.RGBAAt(x, y)
				worst = max(worst, absDiff(a.R, b.R), absDiff(a.G, b.G), absDiff(a.B, b.B))
			}
		}

		t.Run("it should fit them in one GIF palette", func(t *testing.T) {
			assert.LessOrEqual(t, len(p.Palette), 256)
		})

		t.Run("it should keep every pixel within a shade of its colour", func(t *testing.T) {
			assert.LessOrEqual(t, worst, 24)
		})
	})
}
