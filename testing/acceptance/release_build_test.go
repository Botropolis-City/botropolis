package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 167: the city's frame tooling is built only with the devtools tag,
// so release builds neither show nor accept it. The acceptance suite runs
// a devtools build, which is why this builds the release one itself: a
// tag that strips nothing would pass every other test.
func TestReleaseBuildHasNoFrameTooling(t *testing.T) {
	asked := []string{"city", "--screenshot", "frame.png", "--help"}

	t.Run("when botropolis is built without devtools", func(t *testing.T) {
		release := filepath.Join(t.TempDir(), "botropolis")
		out, err := exec.Command("go", "build", "-o", release, "../../cmd/botropolis").CombinedOutput()
		require.NoError(t, err, "%s", out)
		out, err = exec.Command(release, asked...).CombinedOutput()

		t.Run("and city --screenshot is asked for, it should refuse it as an unknown flag", func(t *testing.T) {
			assert.Contains(t, string(out), "unknown flag: --screenshot", "err=%v", err)
		})
	})

	t.Run("when botropolis is built with devtools", func(t *testing.T) {
		out, err := exec.Command(botropolis, asked...).CombinedOutput()

		t.Run("and city --screenshot is asked for, it should accept it", func(t *testing.T) {
			assert.NoError(t, err, "%s", out)
		})
	})
}
