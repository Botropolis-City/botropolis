package render

import (
	"testing"
	"time"

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
		g := &Game{scene: city.NewScene(city.NewLayout()), record: "/frames", started: started, ticks: 90}

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
		g := &Game{scene: city.NewScene(city.NewLayout()), started: started, ticks: 90}

		t.Run("it should follow the wall clock", func(t *testing.T) {
			assert.Equal(t, 60.0, g.clock())
		})
	})
}
