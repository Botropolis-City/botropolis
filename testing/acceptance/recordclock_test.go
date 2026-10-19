package acceptance_test

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 113: under software GL Ebitengine ran several updates per drawn
// frame, so the city's clock, which counts updates, ran about three
// times ahead of the clip, which counts frames.
func TestRecordedClipKeepsTheScenesTime(t *testing.T) {
	helpers.NeedXvfb(t)

	t.Run("when a 3-second clip is recorded at 10 fps", func(t *testing.T) {
		r := newStateRun(t)
		withSession(t, r.home)
		frames := filepath.Join(r.tmp, "frames")
		out, err := r.command(t, false, "--record", frames, "--seconds", "3", "--fps", "10").CombinedOutput()
		require.NoError(t, err, string(out))
		written, _ := filepath.Glob(filepath.Join(frames, "frame-*.png"))
		report := regexp.MustCompile(`recorded (\d+) frames: ([0-9.]+) s of the city's clock`).FindStringSubmatch(string(out))
		require.Len(t, report, 3, string(out))
		seconds, err := strconv.ParseFloat(report[2], 64)
		require.NoError(t, err)

		t.Run("it should write 30 frames", func(t *testing.T) {
			assert.Len(t, written, 30)
		})

		t.Run("it should advance the scene's clock by 3 s, within one tick", func(t *testing.T) {
			assert.InDelta(t, 3.0, seconds, 1.0/30, strings.TrimSpace(report[0]))
		})
	})
}
