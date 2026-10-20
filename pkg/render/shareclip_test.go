package render

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 168, slice 3: a clip of the shared city, of a chosen length, as
// an MP4 that ffmpeg encodes from frames on the city's own clock.

// fakeFFmpeg is a stand-in for ffmpeg that writes down its arguments,
// counts the bytes it is sent, and leaves the file it was asked for.
const fakeFFmpeg = `#!/bin/sh
dir=$(dirname "$0")
if [ "$2" = "-encoders" ]; then
  cat "$dir/encoders"
  exit 0
fi
printf '%s\n' "$@" > "$dir/args"
wc -c > "$dir/bytes"
for last; do :; done
: > "$last"
`

func withFakeFFmpeg(t *testing.T, r *shareRig) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "ffmpeg")
	require.NoError(t, os.WriteFile(bin, []byte(fakeFFmpeg), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "encoders"), []byte(" V....D libx264              H.264\n"), 0o644))
	r.g.findFFmpeg = func() (string, error) { return bin, nil }
	return dir
}

func TestClipFormat(t *testing.T) {
	line := func(name string) string { return " V....D " + name + "             a codec\n" }
	for _, c := range []struct {
		name     string
		encoders string
		want     clipFormat
		ok       bool
	}{
		{"an ffmpeg with libx264", line("libx264") + line("libvpx-vp9"), clipFormat{codec: "libx264", ext: "mp4"}, true},
		{"an ffmpeg with only VP9, as Fedora's ffmpeg-free", line("libvpx-vp9") + line("libopenh264"), clipFormat{codec: "libvpx-vp9", ext: "webm"}, true},
		{"an ffmpeg with neither", line("mpeg4"), clipFormat{}, false},
		{"an ffmpeg that names libx264 only in a description", " V....D h264_v4l2m2m   (codec h264) libx264 replacement\n", clipFormat{}, false},
	} {
		t.Run("when "+c.name+" is asked for a clip", func(t *testing.T) {
			got, ok := pickClipFormat(c.encoders)

			t.Run("it should choose its encoder and container", func(t *testing.T) {
				assert.Equal(t, []any{c.want, c.ok}, []any{got, ok})
			})
		})
	}
}

func TestShareClip(t *testing.T) {
	t.Run("when sharing, l steps the clip's length", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		var lengths []int
		for range len(clipLengths) {
			lengths = append(lengths, r.g.share.clipLength())
			r.press(t, ebiten.KeyL, false)
		}

		t.Run("it should start at 15 seconds and step through 30, 60 and 5", func(t *testing.T) {
			assert.Equal(t, []int{15, 30, 60, 5}, lengths)
		})

		t.Run("it should come back round to 15", func(t *testing.T) {
			assert.Equal(t, 15, r.g.share.clipLength())
		})
	})

	t.Run("when sharing, a 5 second clip is chosen, c is pressed and the city draws on", func(t *testing.T) {
		r := newShareRig(t)
		fake := withFakeFFmpeg(t, r)
		r.press(t, ebiten.KeyS, true)
		for i := 0; r.g.share.clipLength() != 5 && i < len(clipLengths); i++ {
			r.press(t, ebiten.KeyL, false)
		}
		r.press(t, ebiten.KeyC, false)
		recording := r.g.share.recording()
		for i := 0; r.g.share.rec != nil && i < 1000; i++ {
			r.draw()
		}
		r.g.share.encoding.Wait()
		files, err := filepath.Glob(filepath.Join(r.dir, "botropolis-share-*.mp4"))
		require.NoError(t, err)
		args, _ := os.ReadFile(filepath.Join(fake, "args"))
		bytes, _ := os.ReadFile(filepath.Join(fake, "bytes"))
		sent, _ := strconv.Atoi(strings.TrimSpace(string(bytes)))

		t.Run("it should record", func(t *testing.T) {
			assert.True(t, recording)
		})

		t.Run("it should save one MP4 in the videos folder", func(t *testing.T) {
			assert.Len(t, files, 1)
		})

		t.Run("it should send ffmpeg five seconds of frames at thirty a second", func(t *testing.T) {
			assert.Equal(t, 5*clipFPS*shareWidth*shareHeight*4, sent)
		})

		t.Run("it should tell ffmpeg the frames' size", func(t *testing.T) {
			assert.Contains(t, string(args), fmt.Sprintf("-s\n%dx%d\n", shareWidth, shareHeight))
		})

		t.Run("it should tell ffmpeg the frame rate", func(t *testing.T) {
			assert.Contains(t, string(args), fmt.Sprintf("-r\n%d\n", clipFPS))
		})

		t.Run("it should span five seconds of the city's clock, within a tick", func(t *testing.T) {
			assert.InDelta(t, 5.0, r.g.share.last.seconds(), 1.0/liveTPS)
		})

		t.Run("it should say where it saved", func(t *testing.T) {
			require.Len(t, files, 1)
			assert.Contains(t, r.g.notice.Text(timeNow()), files[0])
		})
	})

	t.Run("when sharing and c is pressed without ffmpeg", func(t *testing.T) {
		r := newShareRig(t)
		r.g.findFFmpeg = func() (string, error) { return "", errors.New("not found") }
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyC, false)
		r.draw()
		files, err := filepath.Glob(filepath.Join(r.dir, "botropolis-share-*"))
		require.NoError(t, err)

		t.Run("it should say ffmpeg is missing", func(t *testing.T) {
			assert.Contains(t, r.g.notice.Text(timeNow()), "needs ffmpeg")
		})

		t.Run("it should not record", func(t *testing.T) {
			assert.False(t, r.g.share.recording())
		})

		t.Run("it should save nothing", func(t *testing.T) {
			assert.Empty(t, files)
		})
	})

	t.Run("when sharing and c is pressed with an ffmpeg that has neither encoder", func(t *testing.T) {
		r := newShareRig(t)
		fake := withFakeFFmpeg(t, r)
		require.NoError(t, os.WriteFile(filepath.Join(fake, "encoders"), []byte(" V....D mpeg4                MPEG-4 part 2\n"), 0o644))
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyC, false)

		t.Run("it should name the encoders it needs", func(t *testing.T) {
			note := r.g.notice.Text(timeNow())
			assert.True(t, strings.Contains(note, "libx264") && strings.Contains(note, "libvpx-vp9"), "said %q", note)
		})

		t.Run("it should not record", func(t *testing.T) {
			assert.False(t, r.g.share.recording())
		})
	})

	t.Run("when a clip is recording and esc is pressed", func(t *testing.T) {
		r := newShareRig(t)
		withFakeFFmpeg(t, r)
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyC, false)
		share := r.g.share
		for range 10 {
			r.draw()
		}
		r.press(t, ebiten.KeyEscape, false)
		share.encoding.Wait()
		files, err := filepath.Glob(filepath.Join(r.dir, "botropolis-share-*"))
		require.NoError(t, err)

		t.Run("it should leave no file behind, finished or not", func(t *testing.T) {
			assert.Empty(t, files)
		})
	})
}
