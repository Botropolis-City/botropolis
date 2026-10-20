package acceptance_test

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

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
