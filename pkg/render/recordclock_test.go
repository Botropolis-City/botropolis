package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// A recording is written a frame every few ticks, and on software GL a
// tick takes three or four times as long as it should. Animations that
// read the wall clock then ran three or four times too fast on film and
// jumped between frames: the city looked like it was shaking. While
// recording, time is counted in ticks.

func TestRecordingClock(t *testing.T) {
	started := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)

	t.Run("when recording and ninety ticks have passed", func(t *testing.T) {
		g := &Game{scene: city.NewScene(city.NewLayout()), record: "/frames", started: started, ticks: 90, liveliness: city.Lively}

		t.Run("it should animate as if three seconds had passed", func(t *testing.T) {
			assert.Equal(t, 3.0, g.clock())
		})

		t.Run("it should tell the scene it is three seconds later", func(t *testing.T) {
			assert.Equal(t, started.Add(3*time.Second), g.now())
		})
	})

	t.Run("when not recording", func(t *testing.T) {
		restore := timeNow
		timeNow = func() time.Time { return started.Add(time.Minute) }
		t.Cleanup(func() { timeNow = restore })
		g := &Game{scene: city.NewScene(city.NewLayout()), started: started, ticks: 90, liveliness: city.Lively}

		t.Run("it should follow the wall clock", func(t *testing.T) {
			assert.Equal(t, 60.0, g.clock())
		})
	})
}

// Item 113: the clip counts draws and the city's clock counts updates,
// so while recording they run one to one. Software GL ran several
// updates per draw and a fast display drew more than once per update.
func TestRecordingLockstep(t *testing.T) {
	t.Run("when recording, and an update has run that no draw has drawn", func(t *testing.T) {
		g := &Game{record: "/frames", ticks: 7, painted: 6}

		t.Run("it should hold the next update", func(t *testing.T) {
			assert.False(t, g.startTick())
		})
	})

	t.Run("when recording, and the last update has been drawn", func(t *testing.T) {
		g := &Game{record: "/frames", ticks: 7, painted: 7}

		t.Run("it should run the next update", func(t *testing.T) {
			assert.True(t, g.startTick())
		})
	})

	t.Run("when not recording, and an update has run that no draw has drawn", func(t *testing.T) {
		g := &Game{ticks: 7, painted: 6}

		t.Run("it should run the next update", func(t *testing.T) {
			assert.True(t, g.startTick())
		})
	})

	t.Run("when recording, and a draw comes with no update since the last one", func(t *testing.T) {
		g := &Game{record: "/frames", ticks: 7, painted: 7}

		t.Run("it should count nothing toward the clip", func(t *testing.T) {
			assert.False(t, g.startPaint())
		})
	})

	t.Run("when recording at 10 fps, and the ticks for one written frame have run in lockstep", func(t *testing.T) {
		started := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		g := &Game{scene: city.NewScene(city.NewLayout()), record: "/frames", recordEvery: 3, started: started}
		before := g.now()
		for drawn := 0; drawn < g.recordEvery; {
			for range 3 {
				g.startTick()
			}
			if g.startPaint() {
				drawn++
			}
			g.startPaint()
		}

		t.Run("it should have advanced the clock by exactly 1/10 s", func(t *testing.T) {
			assert.Equal(t, 100*time.Millisecond, g.now().Sub(before))
		})
	})

	t.Run("when recording", func(t *testing.T) {
		g := &Game{scene: city.NewScene(city.NewLayout()), record: "/frames"}
		g.idle()
		t.Cleanup(func() { ebiten.SetTPS(liveTPS) })

		t.Run("it should ask Ebitengine for one update per draw", func(t *testing.T) {
			assert.Equal(t, ebiten.SyncWithFPS, g.tps)
		})
	})
}
