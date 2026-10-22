package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
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
