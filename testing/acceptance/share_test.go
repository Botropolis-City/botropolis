package acceptance_test

import (
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 168, slice 2: shift+s, then g, records a GIF; --record keeps the
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

		t.Run("it should hold sixty frames", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Len(t, anim.Image, 60)
		})

		t.Run("it should be 720 pixels wide", func(t *testing.T) {
			require.NotNil(t, anim)
			assert.Equal(t, 720, anim.Config.Width)
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
