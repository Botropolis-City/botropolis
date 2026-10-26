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
	counts := []struct {
		name    string
		of      func(helpers.Frame) int
		target  int
		ceiling map[int]int
		// ceilingAt is the item the ceiling was last measured at.
		ceilingAt string
		// unmet is, by hour, whether the target is not met yet, and
		// unmatched whether the count is over 1.25× the hamlet's.
		unmet, unmatched map[int]bool
	}{
		{"indices", func(f helpers.Frame) int { return f.Indices }, 20_000, map[int]int{14: 4_990, 22: 5_860}, "item 193", nil, nil},
		// Item 221 raised these from 23 and 39: a ground tree is drawn
		// through a colour matrix, which cannot share a batch with the
		// plain sprites beside it, and costs 2 commands by day and 3 by
		// night where movers are drawn again. Item 223 re-measured them at
		// 26 and 43 on a changed scene, not changed code: the metropolis
		// stopped staging every session as opened together, and lost its
		// false groups. Item 236 brought them back down to 23 and 40: a
		// sprite drawn below half size samples a mipmap level of its
		// page, a texture of its own, and batching now knows it. Item 222
		// set them to 24 and 41, as item 236 measured it: a vehicle's new
		// place in the depth sort splits one batch by day and one by night.
		// Item 236 then drew sprites without mipmaps (Aria, 2026-10-24),
		// and cut their silhouettes out of the lights the same way, and
		// they are 21 and 37. Item 236 then batched the night's lights
		// laid again where movers are, and judged the worst of every
		// settled frame rather than the first second's, which had ended
		// just before a run of 40s: 20 and 37, on item 240's scene.
		{"draw commands", func(f helpers.Frame) int { return f.Commands }, 70, map[int]int{14: 20, 22: 37}, "item 236", nil, nil},
		{"render-target switches", func(f helpers.Frame) int { return f.Switches }, 30, map[int]int{14: 2, 22: 4}, "item 193", nil, nil},
	}

	for _, light := range []struct {
		name string
		hour int
		// runs is how many times the frame is drawn: the night's
		// draw count once varied by run with where Ebitengine packed
		// the light cuts' stamps (item 199).
		runs int
		// wobble is how far the draw count moves between settled
		// frames, as measured (item 236).
		wobble int
	}{
		// 20 to 20 over 104 settled frames, held to 1: a slower machine
		// logs fewer frames, and so samples other places along the lanes.
		{"day", 14, 1, 1},
		// 35 to 37 over 416 settled frames: the night's lights laid
		// again alternate where a window, a sprite in front of it and a
		// window in front of that must keep their order, which is depth
		// and not batching, so fewer would change pixels.
		{"night", 22, 4, 2},
	} {
		t.Run("when the metropolis demo is drawn at 1600×900 by "+light.name+", and it has settled", func(t *testing.T) {
			t.Parallel()
			settled := frameWorkRuns(t, "metropolis", light.hour, light.runs)
			runs := make([]helpers.Frame, len(settled))
			var each []int
			for i, frames := range settled {
				runs[i] = settledWork(t, frames)
				for _, f := range frames {
					each = append(each, f.Commands)
				}
			}
			worst := helpers.MostWork(runs)

			// Item 236: vehicles drive along their lanes through the
			// settled frames, and where one stood once decided which
			// mipmap level a redrawn rectangle pulled in, so the count
			// moved from frame to frame. With mipmaps off, and the
			// night's lights batched, it moves with how many rectangles
			// a mover is drawn again in and what they hold, by as much as
			// each light's wobble.
			spread := slices.Max(each) - slices.Min(each)
			t.Logf("draw commands in the %d settled frames: %d to %d", len(each), slices.Min(each), slices.Max(each))
			t.Run(fmt.Sprintf("it should vary by no more than %d draw commands between settled frames, as the vehicles move along their lanes", light.wobble), func(t *testing.T) {
				if !assert.LessOrEqual(t, spread, light.wobble) {
					logOffCount(t, settled)
				}
			})

			if light.runs > 1 {
				commands := make([]int, len(runs))
				for i, r := range runs {
					commands[i] = r.Commands
				}
				t.Logf("draw commands in each of %d runs: %v", light.runs, commands)

				t.Run(fmt.Sprintf("it should issue the same number of draw commands in each of %d runs", light.runs), func(t *testing.T) {
					assert.Len(t, slices.Compact(slices.Sorted(slices.Values(commands))), 1)
				})
			}

			for _, c := range counts {
				got := c.of(worst)
				t.Logf("%s: %d a frame; ceiling %d, target %d", c.name, got, c.ceiling[light.hour], c.target)

				t.Run(fmt.Sprintf("it should issue no more %s a frame than it did at %s", c.name, c.ceilingAt), func(t *testing.T) {
					assert.LessOrEqual(t, got, c.ceiling[light.hour])
				})

				t.Run(fmt.Sprintf("it should issue no more than %d %s a frame", c.target, c.name), func(t *testing.T) {
					if c.unmet[light.hour] {
						require.Greater(t, got, c.target, "target met: remove the skip")
						t.Skipf("unmet: %d against target %d; item 193", got, c.target)
					}
					assert.LessOrEqual(t, got, c.target)
				})
			}
		})

		// Item 193: once the standing city is composed, a city with
		// twenty districts costs about what one with three does, for the
		// same activity. Hold movers equal (Aria, 2026-10-21): both are
		// drawn with none — no travellers, and no building animating for
		// its session — so the comparison is of the still city and the
		// chrome, and more activity may cost more.
		t.Run("when the hamlet and the metropolis are drawn by "+light.name+" with the same movers, none: no travellers and no building animating", func(t *testing.T) {
			t.Parallel()
			metropolis := frameWorkWith(t, "metropolis", light.hour, withoutMovers, allSighted)
			hamlet := frameWorkWith(t, "hamlet", light.hour, withoutMovers, allSighted)

			for _, c := range counts {
				got, limit := c.of(metropolis), c.of(hamlet)*5/4
				t.Logf("%s: the metropolis %d a frame, the hamlet %d", c.name, got, c.of(hamlet))

				t.Run(fmt.Sprintf("it should cost the metropolis no more than 1.25× the hamlet in %s", c.name), func(t *testing.T) {
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

// withoutMovers is the environment that has the debug city leave out
// everything that moves (item 193).
const withoutMovers = "BOTROPOLIS_FRAME_MOVERS=none"

// allSighted is the environment that has the debug city start with every
// kind captioned already. A city that has never sighted one captions it
// for 6 seconds and the next 2 seconds later, and each caption coming
// down redraws the map's labels: 27 and 29 commands where a settled frame
// takes 19 (item 260). A settled city is an everyday one, and a caption's
// cost is its own tests'.
const allSighted = "BOTROPOLIS_FRAME_SIGHTED=all"

// settledFrom is the first frame counted. The buildings' looks and the
// map's labels are baked by the third frame, onto pages of their own that
// Ebitengine never moves (items 192 and 193), and the counts hold from
// then; the margin is the camera's settling. It counts from the city's
// first frame, as the choreographies' clock does: counted from the first
// frame logged, it moved with how long the art took to go up, and was the
// 20th logged when that took three frames (item 162).
const settledFrom = 17

// settledFrames runs the debug city for a second of frames on a staged
// demo, with the local clock at hour, and returns the settled frames.
func settledFrames(t *testing.T, demo string, hour int, env ...string) []helpers.Frame {
	t.Helper()
	all := frameLog(t, demo, hour, 1, "", env...)
	require.Greater(t, len(all), settledFrom+10, "too few frames logged to call any settled")
	return all[settledFrom:]
}

// Item 236: a second of frames ended just before the night's worst run
// of them, cars passing lit buildings 20 frames after the cut, so the
// ceiling held by timing. The work is judged over the 104 settled frames
// the performance manager measured.
//
// Item 260: they are judged by the city's tick, not by how many frames
// came. A recording advances its clock one tick a frame however busy the
// machine (item 113), so 4 seconds always held exactly these 104 frames,
// with none to spare. The record then doubled to 8 seconds whenever one
// went missing, and counted what the city does after 4 seconds, which
// these limits were never measured on (item 261). So the city records 5
// seconds, and the frames judged are the 104 ticks from settledFrom on
// every machine: load lengthens the wait, never the span.
const (
	recordSettled = 104
	recordSeconds = 5.0
)

// recordedSettledFrames runs the debug city on a staged demo, with the
// local clock at hour, and returns the recordSettled settled frames from
// tick settledFrom after its first.
func recordedSettledFrames(t *testing.T, demo string, hour int) []helpers.Frame {
	t.Helper()
	all := frameLog(t, demo, hour, recordSeconds, "", allSighted)
	require.NotEmpty(t, all, "the city drew no frame")
	from := all[0].Tick + settledFrom
	settled := slices.DeleteFunc(slices.Clone(all), func(f helpers.Frame) bool {
		return f.Tick < from || f.Tick >= from+recordSettled
	})
	require.Len(t, settled, recordSettled, "the frames of ticks %d to %d, in %g s of the city", from, from+recordSettled-1, recordSeconds)
	return settled
}

// logOffCount logs every settled frame whose draw count is not the most
// common one, with what its standing city and looks did, so a spread on
// a runner nobody can watch says which frames carry it (item 260).
func logOffCount(t *testing.T, runs [][]helpers.Frame) {
	t.Helper()
	seen := map[int]int{}
	for _, frames := range runs {
		for _, f := range frames {
			seen[f.Commands]++
		}
	}
	common := 0
	for commands, n := range seen {
		if n > seen[common] {
			common = commands
		}
	}
	for r, frames := range runs {
		for _, f := range frames {
			if f.Commands != common {
				t.Logf("run %d, tick %d: %d draw commands, not %d; still %+v, looks %+v", r+1, f.Tick, f.Commands, common, f.Still, f.Looks)
			}
		}
	}
}

// frameWorkRuns runs the debug city n times at once, each in its own
// subtest, so a failed run fails only itself, and returns each run's
// settled frames.
func frameWorkRuns(t *testing.T, demo string, hour, n int) [][]helpers.Frame {
	t.Helper()
	frames := make([][]helpers.Frame, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			t.Run(fmt.Sprintf("run %d", i+1), func(t *testing.T) {
				frames[i] = recordedSettledFrames(t, demo, hour)
			})
		})
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	return frames
}

// frameWorkWith runs the debug city for a second of frames on a staged
// demo, with the local clock at hour and more of its environment, and
// returns each count's worst over the settled frames.
func frameWorkWith(t *testing.T, demo string, hour int, env ...string) helpers.Frame {
	t.Helper()
	return settledWork(t, settledFrames(t, demo, hour, env...))
}

// settledWork is each count's worst over frames that should have
// settled, logging every look that missed in them with what changed in
// its key, so a failure on a runner nobody can watch says why (item 202).
func settledWork(t *testing.T, frames []helpers.Frame) helpers.Frame {
	t.Helper()
	for _, f := range frames {
		for _, miss := range f.Missed {
			t.Log(miss)
		}
	}
	return helpers.MostWork(frames)
}

// frameLog runs the debug city for seconds of frames on a staged demo,
// with the local clock at hour, playing a choreography if one is given
// as JSON, and returns every frame it logged.
func frameLog(t *testing.T, demo string, hour int, seconds float64, choreography string, env ...string) []helpers.Frame {
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
	cmd.Env = append(append(os.Environ(), "TZ="+zoneAt(time.Now().UTC().Hour(), hour)), env...)
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
	// Item 202: a city that opened at its ceiling, on a history not yet
	// read, has its heights still to change, so nothing counted from it
	// says what a settled frame costs.
	require.False(t, helpers.OpenedEarly(all), "the city opened at its ceiling, on a history not yet read: %s", stderr.String())
	return helpers.CityFrames(all)
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
			worst := settledWork(t, all[view.from:])
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
				worst := settledWork(t, all[view.from:])
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
