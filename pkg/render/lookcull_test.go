package render

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/testing/helpers"
)

// fullZoom is a city of 217 sessions, about the real history's, settled
// at full zoom on a 1920×1200 window, where at r905 the looks of the
// buildings off screen filled the look pages and those on screen were
// drawn straight every frame (item 161).
func fullZoom(t testing.TB, opts DriveOptions) *Drive {
	t.Helper()
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
	opts.Home, opts.Now, opts.Width, opts.Height = helpers.ParkedWeek(t, 217, now, "Full zoom"), now, 1920, 1200
	d, err := NewDrive(opts)
	require.NoError(t, err)
	require.NoError(t, d.Run(30))
	d.game.scene.Wheel(city.Point{X: 960, Y: 600}, 40)
	require.NoError(t, d.Run(240))
	return d
}

func TestFullZoomDrawsWhatIsOnScreen(t *testing.T) {
	t.Run("when a city of 217 sessions has settled at full zoom", func(t *testing.T) {
		d := fullZoom(t, DriveOptions{})
		var frames []lookStats
		for range 10 {
			require.NoError(t, d.Tick())
			frames = append(frames, d.game.lookStats)
		}

		t.Run("it should draw fewer than half of its buildings", func(t *testing.T) {
			assert.Less(t, d.game.buildingsDrawn, len(d.game.scene.City().Buildings())/2)
		})

		t.Run("and 10 frames pass", func(t *testing.T) {
			bakes, misses := 0, 0
			for _, s := range frames {
				bakes += s.bakes
				misses += s.misses
			}

			t.Run("it should bake no look on any of them", func(t *testing.T) {
				assert.Zero(t, bakes)
			})

			// A ceiling against regressions, not a target: the visible
			// looks are about 27 MB in a 32 MiB cap, and shelf packing
			// leaves no room for the last 8 of 46; the target is the
			// frame budget, measured on the real history.
			t.Run("it should draw no more looks straight than at slice 2 (8 a frame)", func(t *testing.T) {
				assert.LessOrEqual(t, misses, 8*len(frames))
			})
		})

		t.Run("it should count every look it draws as a hit, a bake or a miss", func(t *testing.T) {
			var uncounted []lookStats
			for _, s := range frames {
				if s.runs != s.hits+s.bakes+s.misses {
					uncounted = append(uncounted, s)
				}
			}
			assert.Empty(t, uncounted)
		})
	})

	t.Run("when it is drawn straight", func(t *testing.T) {
		d := fullZoom(t, DriveOptions{Straight: true})

		t.Run("it should draw every building", func(t *testing.T) {
			assert.Equal(t, len(d.game.scene.City().Buildings()), d.game.buildingsDrawn)
		})
	})
}

// A building is left undrawn when its cull bound misses the frame, so
// the bound must hold everything the building paints, its lights with
// it: each building is recorded whole, as the dry pass records it, and
// the box it inks must lie inside its bound.
func TestCullBoundsHoldWhatABuildingPaints(t *testing.T) {
	day := time.Now()
	for _, c := range []struct {
		name string
		open func(t *testing.T) *Drive
	}{
		{"217 sessions at full zoom", func(t *testing.T) *Drive { return fullZoom(t, DriveOptions{}) }},
		{"the metropolis at full zoom by day", func(t *testing.T) *Drive { return demoFullZoom(t, day, 14) }},
		{"the metropolis at full zoom by night", func(t *testing.T) *Drive { return demoFullZoom(t, day, 22) }},
		{"the metropolis at fit by night", func(t *testing.T) *Drive {
			d, err := NewDrive(DriveOptions{Home: stageMetropolis(t, day), Now: atHour(day, 22), Width: 1920, Height: 1200})
			require.NoError(t, err)
			require.NoError(t, d.Run(120))
			return d
		}},
	} {
		t.Run("when "+c.name+" is recorded building by building", func(t *testing.T) {
			d := c.open(t)
			outside := d.game.paintedOutsideBounds()

			t.Run("it should paint nothing outside any building's cull bound", func(t *testing.T) {
				assert.Empty(t, outside)
			})
		})
	}
}

// paintedOutsideBounds records every building of the last frame's city
// whole, through the dry pass, and names each that inked outside its
// cull bound, with both boxes.
func (g *Game) paintedOutsideBounds() []string {
	cam := g.scene.Camera()
	selected, detailed, seconds := g.scene.Selected(), g.scene.Detailed(), g.clock()
	var out []string
	for _, b := range g.scene.City().Buildings() {
		bound, ok := g.cullBound(cam, b, b == selected, detailed, seconds)
		if !ok {
			continue
		}
		g.note(func() { g.drawBuilding(g.rec.dry, cam, b, b == selected, detailed, seconds) }, false)
		box := g.still.entries[len(g.still.entries)-1].box
		if box.Empty() {
			continue
		}
		if float64(box.Min.X) < math.Floor(bound.Min.X) || float64(box.Min.Y) < math.Floor(bound.Min.Y) ||
			float64(box.Max.X) > math.Ceil(bound.Max.X) || float64(box.Max.Y) > math.Ceil(bound.Max.Y) {
			out = append(out, fmt.Sprintf("%s: inked %v, bound %v", b.Session.ID, box, bound))
		}
	}
	return out
}

func atHour(day time.Time, hour int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, 30, 0, 0, time.Local)
}

// stageMetropolis stages the largest demo as if it were at a moment.
func stageMetropolis(t *testing.T, at time.Time) string {
	t.Helper()
	scenario, err := demo.LoadScenario(filepath.Join("..", "..", "demo", "scenarios", "metropolis.yaml"))
	require.NoError(t, err)
	home := t.TempDir()
	corpus := demo.Corpus{Dir: filepath.Join("..", "..", "demo", "corpus")}
	require.NoError(t, demo.Stage(corpus, scenario, home, at, demo.Sleepers(home, 10*time.Minute)))
	t.Cleanup(func() { _ = demo.Unstage(home) })
	return home
}

func demoFullZoom(t *testing.T, day time.Time, hour int) *Drive {
	t.Helper()
	d, err := NewDrive(DriveOptions{Home: stageMetropolis(t, day), Now: atHour(day, hour), Width: 1920, Height: 1200})
	require.NoError(t, err)
	require.NoError(t, d.Run(30))
	d.game.scene.Wheel(city.Point{X: 960, Y: 600}, 40)
	require.NoError(t, d.Run(240))
	return d
}

// The time to update and draw a frame of the full-zoom city, and what
// its looks did, for item 161's parts to be compared by. It reports and
// never gates.
func BenchmarkAFullZoomFrame(b *testing.B) {
	d := fullZoom(b, DriveOptions{})
	var s lookStats
	for b.Loop() {
		require.NoError(b, d.Tick())
		s = d.game.lookStats
	}
	b.ReportMetric(float64(s.runs), "runs/frame")
	b.ReportMetric(float64(s.misses), "misses/frame")
	b.ReportMetric(float64(s.bakes), "bakes/frame")
}
