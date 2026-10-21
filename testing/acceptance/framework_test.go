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
	const ceilingAt = "item 193"
	counts := []struct {
		name    string
		of      func(helpers.Frame) int
		target  int
		ceiling map[int]int
		// unmet is, by hour, whether the target is not met yet, and
		// unmatched whether the count is over 1.25× the hamlet's.
		unmet, unmatched map[int]bool
	}{
		{"indices", func(f helpers.Frame) int { return f.Indices }, 20_000, map[int]int{14: 5_340, 22: 6_210}, nil, map[int]bool{22: true}},
		{"draw commands", func(f helpers.Frame) int { return f.Commands }, 70, map[int]int{14: 35, 22: 52}, nil, map[int]bool{14: true, 22: true}},
		{"render-target switches", func(f helpers.Frame) int { return f.Switches }, 30, map[int]int{14: 4, 22: 6}, nil, map[int]bool{14: true, 22: true}},
	}

	for _, light := range []struct {
		name string
		hour int
	}{{"day", 14}, {"night", 22}} {
		t.Run("when the metropolis demo is drawn at 1600×900 by "+light.name+", and it has settled", func(t *testing.T) {
			t.Parallel()
			worst := frameWork(t, "metropolis", light.hour)
			hamlet := frameWork(t, "hamlet", light.hour)

			for _, c := range counts {
				got := c.of(worst)
				t.Logf("%s: %d a frame; ceiling %d, target %d; the hamlet %d", c.name, got, c.ceiling[light.hour], c.target, c.of(hamlet))

				t.Run(fmt.Sprintf("it should issue no more %s a frame than it did at %s", c.name, ceilingAt), func(t *testing.T) {
					assert.LessOrEqual(t, got, c.ceiling[light.hour])
				})

				t.Run(fmt.Sprintf("it should issue no more than %d %s a frame", c.target, c.name), func(t *testing.T) {
					if c.unmet[light.hour] {
						require.Greater(t, got, c.target, "target met: remove the skip")
						t.Skipf("unmet: %d against target %d; item 193", got, c.target)
					}
					assert.LessOrEqual(t, got, c.target)
				})

				// Item 193: once the standing city is composed, what a
				// frame costs is what moves and the chrome, so the city
				// with twenty districts costs about what the one with
				// three does.
				t.Run(fmt.Sprintf("it should issue no more than 1.25× the hamlet's %s a frame", c.name), func(t *testing.T) {
					limit := c.of(hamlet) * 5 / 4
					if c.unmatched[light.hour] {
						require.Greater(t, got, limit, "target met: remove the skip")
						t.Skipf("unmet: %d against %d, 1.25× the hamlet's %d; item 193", got, limit, c.of(hamlet))
					}
					assert.LessOrEqual(t, got, limit)
				})
			}
		})
	}
}

// settledFrom is the first frame counted. The buildings' looks are baked
// by the third frame, and about ten frames after that Ebitengine moves
// each onto its shared atlas, a copy each, once (item 192); from then the
// blits batch together and the counts hold.
const settledFrom = 20

// frameWork runs the debug city for a second of frames on a staged demo,
// with the local clock at hour, and returns each count's worst over the
// settled frames.
func frameWork(t *testing.T, demo string, hour int) helpers.Frame {
	t.Helper()
	all := frameLog(t, demo, hour, 1, "")
	require.Greater(t, len(all), settledFrom+10, "too few frames logged to call any settled")
	return helpers.MostWork(all[settledFrom:])
}

// frameLog runs the debug city for seconds of frames on a staged demo,
// with the local clock at hour, playing a choreography if one is given
// as JSON, and returns every frame it logged.
func frameLog(t *testing.T, demo string, hour int, seconds float64, choreography string) []helpers.Frame {
	t.Helper()
	bin, err := debugCity()
	require.NoError(t, err)
	home := stageDemo(t, demo)
	frames := filepath.Join(t.TempDir(), "frames")
	require.NoError(t, os.MkdirAll(frames, 0o755))
	logPath := filepath.Join(t.TempDir(), "commands.log")
	out, err := os.Create(logPath)
	require.NoError(t, err)
	cmd := exec.Command(bin, "city", "--headless", "--record", frames, "--seconds", fmt.Sprint(seconds), "--fps", "1",
		"--window", "1600x900", "--home", home, "--socket", filepath.Join(t.TempDir(), "none.sock"))
	if choreography != "" {
		path := filepath.Join(t.TempDir(), "script.json")
		require.NoError(t, os.WriteFile(path, []byte(choreography), 0o644))
		cmd.Args = append(cmd.Args, "--script", path)
	}
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
	return helpers.ParseFrameLog(f)
}

// Item 192: a still city draws its buildings from looks baked once, so a
// settled frame bakes none, at fit and up close, and the looks it keeps
// fit the cache's cap. While the camera moves a building's look changes
// every frame and is drawn straight, as before, so those frames are
// measured and reported, not gated.
func TestTheLooksAFrameKeeps(t *testing.T) {
	helpers.NeedXvfb(t)
	const lookCap = 32 << 20
	looks := func(f helpers.Looks) string {
		return fmt.Sprintf("hits %d, misses %d, bakes %d, %.1f MiB kept", f.Hits, f.Misses, f.Bakes, float64(f.Bytes)/(1<<20))
	}
	closer := `[{"at":0.1,"key":"Equal"},{"at":0.2,"key":"Equal"},{"at":0.3,"key":"Equal"}]`

	for _, view := range []struct {
		name, choreography string
		seconds            float64
		from               int
	}{
		{"fit", "", 1, settledFrom},
		{"three notches in", closer, 3, 60},
	} {
		t.Run("when the metropolis has settled at "+view.name, func(t *testing.T) {
			t.Parallel()
			all := frameLog(t, "metropolis", 14, view.seconds, view.choreography)
			require.Greater(t, len(all), view.from+10, "too few frames logged to call any settled")
			worst := helpers.MostWork(all[view.from:])
			t.Logf("worst settled frame: %s; %d commands, %d indices, %d switches", looks(worst.Looks), worst.Commands, worst.Indices, worst.Switches)

			t.Run("it should bake nothing a frame", func(t *testing.T) {
				assert.Zero(t, worst.Looks.Bakes)
			})

			t.Run("it should draw its buildings from the cache", func(t *testing.T) {
				assert.Positive(t, worst.Looks.Hits)
			})

			t.Run("it should keep its looks within the cache's cap", func(t *testing.T) {
				assert.LessOrEqual(t, worst.Looks.Bytes, lookCap)
			})
		})
	}

	for _, move := range []struct{ name, choreography string }{
		{"zooms in with the wheel", `[{"at":0.5,"wheel":4,"over":1.5}]`},
		{"pans with an arrow held", `[{"at":0.5,"hold":"ArrowRight","for":1.5}]`},
	} {
		t.Run("when the camera "+move.name, func(t *testing.T) {
			t.Parallel()
			all := frameLog(t, "metropolis", 14, 2.5, move.choreography)
			require.Greater(t, len(all), 60, "too few frames logged")
			moving := helpers.MostWork(all[15:60])
			t.Logf("worst frame while moving: %s; %d commands, %d indices, %d switches", looks(moving.Looks), moving.Commands, moving.Indices, moving.Switches)

			t.Run("it should still draw its buildings", func(t *testing.T) {
				assert.Positive(t, moving.Looks.Hits+moving.Looks.Misses)
			})
		})
	}
}

// Item 193: the standing city is composed once and kept, so a settled
// metropolis composes none of it a frame, at fit and up close: not the
// ground, not the stills, not the group lines, not even a rectangle.
func TestTheStillLayerAFrameKeeps(t *testing.T) {
	helpers.NeedXvfb(t)
	closer := `[{"at":0.1,"key":"Equal"},{"at":0.2,"key":"Equal"},{"at":0.3,"key":"Equal"}]`

	for _, view := range []struct {
		name, choreography string
		seconds            float64
		from               int
	}{
		{"fit", "", 1, settledFrom},
		{"three notches in", closer, 3, 60},
	} {
		for _, light := range []struct {
			name string
			hour int
		}{{"day", 14}, {"night", 22}} {
			t.Run("when the metropolis has settled at "+view.name+" by "+light.name, func(t *testing.T) {
				t.Parallel()
				all := frameLog(t, "metropolis", light.hour, view.seconds, view.choreography)
				require.Greater(t, len(all), view.from+10, "too few frames logged to call any settled")
				worst := helpers.MostWork(all[view.from:])
				t.Logf("worst settled frame: %+v", worst.Still)

				t.Run("it should compose nothing of the standing city a frame", func(t *testing.T) {
					assert.Zero(t, worst.Still.Composes+worst.Still.Redos+worst.Still.Overlays+worst.Still.Ground)
				})
			})
		}
	}

	// A held arrow pans by whole pixels, so the standing city is shifted
	// and only the strips the pan uncovers are composed: never the whole
	// of it, nor drawn straight. The pan's counts are reported against
	// the targets, not gated.
	t.Run("when the camera pans with an arrow held", func(t *testing.T) {
		t.Parallel()
		all := frameLog(t, "metropolis", 14, 2.5, `[{"at":0.5,"hold":"ArrowRight","for":1.5}]`)
		require.Greater(t, len(all), 60, "too few frames logged")
		panning := all[20:55]
		worst := helpers.MostWork(panning)
		t.Logf("worst frame while panning: %+v; %d commands, %d indices, %d switches; targets 70, 20000, 30", worst.Still, worst.Commands, worst.Indices, worst.Switches)

		t.Run("it should shift the standing city", func(t *testing.T) {
			assert.Positive(t, worst.Still.Shifts)
		})

		t.Run("it should compose only strips", func(t *testing.T) {
			assert.Zero(t, worst.Still.Composes+worst.Still.Overlays+worst.Still.Ground+worst.Still.Direct)
		})
	})
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
