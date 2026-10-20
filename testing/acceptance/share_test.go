package acceptance_test

import (
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 168, slice 2: shift+s, then g, records a GIF (at 1080p since item
// 188); --record keeps the
// city running, and presses a key every two seconds, so g comes at 3 s
// and the GIF is done at 7.
func TestSharingGIF(t *testing.T) {
	helpers.NeedXvfb(t)
	home := helpers.LiveFixtureHome(t, "synthetic")
	person := t.TempDir()
	pictures := filepath.Join(person, "Pictures")
	require.NoError(t, os.MkdirAll(pictures, 0o755))
	socket := filepath.Join(t.TempDir(), "none.sock")

	cmd := exec.Command(botropolis, "city", "--headless", "--record", t.TempDir(), "--seconds", "9", "--fps", "1", "--home", home, "--socket", socket, "--keys", "shift+s,g")
	cmd.Env = append(os.Environ(), "HOME="+person, "XDG_CONFIG_HOME="+filepath.Join(person, ".config"))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	saved, err := filepath.Glob(filepath.Join(pictures, "botropolis-share-*.gif"))
	require.NoError(t, err)
	var anim *gif.GIF
	if len(saved) == 1 {
		f, err := os.Open(saved[0])
		require.NoError(t, err)
		anim, err = gif.DecodeAll(f)
		_ = f.Close()
		require.NoError(t, err)
	}

	t.Run("when shift+s and then g are pressed in the city", func(t *testing.T) {
		t.Run("it should save one GIF in the Pictures folder", func(t *testing.T) {
			assert.Len(t, saved, 1)
		})

		t.Run("it should hold 120 frames, four seconds at thirty a second", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Len(t, anim.Image, 120)
		})

		t.Run("it should be 1920 pixels wide", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Equal(t, 1920, anim.Config.Width)
		})

		t.Run("it should be 1080 pixels high", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Equal(t, 1080, anim.Config.Height)
		})
	})
}

// Item 168, slice 3: shift+s, l three times for a 5-second clip, then c,
// encoded by the real ffmpeg. --record presses a key every two seconds,
// so c comes at 9 s and the clip is done at 14.
func TestSharingClip(t *testing.T) {
	helpers.NeedXvfb(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if _, ffmpegErr := exec.LookPath("ffmpeg"); ffmpegErr != nil || err != nil {
		t.Skip("no ffmpeg and ffprobe here to encode and read a clip; the unit tests cover the pipe")
	}
	home := helpers.LiveFixtureHome(t, "synthetic")
	person := t.TempDir()
	videos := filepath.Join(person, "Videos")
	require.NoError(t, os.MkdirAll(videos, 0o755))
	socket := filepath.Join(t.TempDir(), "none.sock")

	cmd := exec.Command(botropolis, "city", "--headless", "--record", t.TempDir(), "--seconds", "16", "--fps", "1", "--home", home, "--socket", socket, "--keys", "shift+s,l,l,l,c")
	cmd.Env = append(os.Environ(), "HOME="+person, "XDG_CONFIG_HOME="+filepath.Join(person, ".config"))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	saved, err := filepath.Glob(filepath.Join(videos, "botropolis-share-*.mp4"))
	require.NoError(t, err)
	probe := func(t *testing.T, entry string) string {
		t.Helper()
		require.Len(t, saved, 1)
		out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-count_frames",
			"-show_entries", entry, "-of", "default=noprint_wrappers=1:nokey=1", saved[0]).Output()
		require.NoError(t, err)
		return strings.TrimSpace(string(out))
	}

	t.Run("when shift+s, a 5-second length and then c are pressed in the city", func(t *testing.T) {
		t.Run("it should save one MP4 in the Videos folder", func(t *testing.T) {
			assert.Len(t, saved, 1)
		})

		t.Run("it should hold five seconds of frames at thirty a second", func(t *testing.T) {
			assert.Equal(t, "150", probe(t, "stream=nb_read_frames"))
		})

		t.Run("it should be H.264, which plays anywhere", func(t *testing.T) {
			assert.Equal(t, "h264", probe(t, "stream=codec_name"))
		})
	})
}

// Item 168: sharing mode, as a person uses it: shift+s, then p.
func TestSharingPicture(t *testing.T) {
	helpers.NeedXvfb(t)
	home := helpers.LiveFixtureHome(t, "synthetic")
	person := t.TempDir()
	pictures := filepath.Join(person, "Pictures")
	require.NoError(t, os.MkdirAll(pictures, 0o755))
	shot := filepath.Join(t.TempDir(), "frame.png")
	socket := filepath.Join(t.TempDir(), "none.sock")

	cmd := exec.Command(botropolis, "city", "--headless", "--screenshot", shot, "--home", home, "--socket", socket, "--keys", "shift+s,p")
	cmd.Env = append(os.Environ(), "HOME="+person, "XDG_CONFIG_HOME="+filepath.Join(person, ".config"))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	saved, err := filepath.Glob(filepath.Join(pictures, "botropolis-share-*.png"))
	require.NoError(t, err)

	t.Run("when shift+s and then p are pressed in the city", func(t *testing.T) {
		t.Run("it should save one picture in the Pictures folder", func(t *testing.T) {
			assert.Len(t, saved, 1)
		})

		t.Run("it should save a PNG", func(t *testing.T) {
			require.Len(t, saved, 1)
			f, err := os.Open(saved[0])
			require.NoError(t, err)
			defer func() { _ = f.Close() }()
			_, err = png.DecodeConfig(f)
			assert.NoError(t, err)
		})
	})
}
