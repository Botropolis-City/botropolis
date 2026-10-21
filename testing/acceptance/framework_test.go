//go:build linux

package acceptance_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 194: the work a frame of the largest demo asks of the GPU, counted
// from Ebitengine's own command log, so a regression in drawing fails a
// test instead of passing every one. Counts, not seconds: they do not
// change with machine load. Indices are the count that moved when the
// heap grew to gigabytes (item 156); commands alone would have passed it.
//
// A driven city cannot count this, because it drops every draw before
// it reaches Ebitengine's queue (pkg/render/drive_deferred.go), so the
// city runs for real, headless, built with ebitenginedebug.
//
// Each count has a target, set from the budget's 10 ms of CPU a frame
// and what items 191–193 can reach, and a ceiling, today's worst plus
// 2%, that gates until the targets are met. Each of 191, 192 and 193
// lowers the ceilings to what it reached. A target case skips, printing
// the gap, while its count is over the target, and fails once it is met,
// so the skip cannot outlive the fix that removes it.
func TestTheWorkAFrameTakes(t *testing.T) {
	helpers.NeedXvfb(t)
	const ceilingAt = "item 191"
	counts := []struct {
		name    string
		of      func(helpers.Frame) int
		target  int
		ceiling map[int]int
		skip    bool
	}{
		{"indices", func(f helpers.Frame) int { return f.Indices }, 20_000, map[int]int{14: 77_100, 22: 80_200}, true},
		{"draw commands", func(f helpers.Frame) int { return f.Commands }, 70, map[int]int{14: 283, 22: 341}, true},
		{"render-target switches", func(f helpers.Frame) int { return f.Switches }, 30, map[int]int{14: 6, 22: 8}, false},
	}

	for _, light := range []struct {
		name string
		hour int
	}{{"day", 14}, {"night", 22}} {
		t.Run("when the metropolis demo is drawn at 1600×900 by "+light.name+", and it has settled", func(t *testing.T) {
			t.Parallel()
			worst := frameWork(t, "metropolis", light.hour)

			for _, c := range counts {
				got := c.of(worst)
				t.Logf("%s: %d a frame; ceiling %d, target %d", c.name, got, c.ceiling[light.hour], c.target)

				t.Run(fmt.Sprintf("it should issue no more %s a frame than it did at %s", c.name, ceilingAt), func(t *testing.T) {
					assert.LessOrEqual(t, got, c.ceiling[light.hour])
				})

				t.Run(fmt.Sprintf("it should issue no more than %d %s a frame", c.target, c.name), func(t *testing.T) {
					if c.skip {
						require.Greater(t, got, c.target, "target met: remove the skip")
						t.Skipf("unmet: %d against target %d; items 191–193", got, c.target)
					}
					assert.LessOrEqual(t, got, c.target)
				})
			}
		})
	}
}

// settledFrom is the first frame counted: the counts settle by the fifth.
const settledFrom = 10

// frameWork runs the debug city for a second of frames on a staged demo,
// with the local clock at hour, and returns each count's worst over the
// settled frames.
func frameWork(t *testing.T, demo string, hour int) helpers.Frame {
	t.Helper()
	bin, err := debugCity()
	require.NoError(t, err)
	home := stageDemo(t, demo)
	frames := filepath.Join(t.TempDir(), "frames")
	require.NoError(t, os.MkdirAll(frames, 0o755))
	logPath := filepath.Join(t.TempDir(), "commands.log")
	out, err := os.Create(logPath)
	require.NoError(t, err)
	cmd := exec.Command(bin, "city", "--headless", "--record", frames, "--seconds", "1", "--fps", "1",
		"--window", "1600x900", "--home", home, "--socket", filepath.Join(t.TempDir(), "none.sock"))
	cmd.Env = append(os.Environ(), "TZ="+zoneAt(time.Now().UTC().Hour(), hour))
	cmd.Stdout = out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	require.NoError(t, out.Close())
	require.NoError(t, runErr, stderr.String())

	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	all := helpers.ParseFrameLog(f)
	require.Greater(t, len(all), settledFrom+10, "too few frames logged to call any settled")
	return helpers.MostWork(all[settledFrom:])
}

// zoneAt names a fixed zone in which the UTC hour utc reads as hour.
func zoneAt(utc, hour int) string {
	d := ((hour-utc)%24 + 24) % 24
	if d > 14 {
		d -= 24
	}
	switch {
	case d > 0:
		return fmt.Sprintf("Etc/GMT-%d", d)
	case d < 0:
		return fmt.Sprintf("Etc/GMT+%d", -d)
	}
	return "Etc/GMT"
}

// debugCity builds the city once with Ebitengine's command log on,
// beside the suite's own binary, which TestMain removes.
var debugCity = sync.OnceValues(func() (string, error) {
	bin := filepath.Join(filepath.Dir(botropolis), "botropolis-framelog")
	build := exec.Command("go", "build", "-tags", "devtools ebitenginedebug", "-o", bin, "../../cmd/botropolis")
	if os.Getenv("BOTROPOLIS_ACCEPTANCE_COVER") != "" {
		build.Args = slices.Insert(build.Args, 2, "-cover", "-coverpkg=botropolis.city/botropolis/pkg/...,botropolis.city/botropolis/cmd/...")
	}
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return bin, nil
})
