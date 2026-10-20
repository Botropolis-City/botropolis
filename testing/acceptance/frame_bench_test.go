package acceptance_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/render"
)

// Item 166: the time to update and draw a frame of the largest demo by
// day and by night. At night each building lays its lit windows in a
// layer and every sprite drawn after it is drawn again to cut its shape
// out of that layer; the gap between the rows is what the night costs.
// It reports and never gates.
func BenchmarkDrawANightFrame(b *testing.B) {
	day := time.Now()
	for _, when := range []struct {
		name string
		hour int
	}{{"day", 14}, {"night", 22}} {
		b.Run("metropolis/fitted/"+when.name, func(b *testing.B) {
			now := time.Date(day.Year(), day.Month(), day.Day(), when.hour, 30, 0, 0, time.Local)
			d, err := render.NewDrive(render.DriveOptions{Home: stageDemo(b, "metropolis"), Width: 1600, Height: 900, Now: now})
			require.NoError(b, err)
			require.NoError(b, d.Run(settleTicks))
			for b.Loop() {
				require.NoError(b, d.Tick())
			}
			shown := d.Shown()
			b.ReportMetric(float64(shown.Sprites), "sprites/frame")
			b.ReportMetric(float64(shown.WindowDraws), "windows/frame")
			b.ReportMetric(float64(shown.Punches), "cuts/frame")
		})
	}
}

// The time to update and draw one frame of the largest demo at 1600 by
// 900, fitted and zoomed in until its labels show, with the labels on
// and turned off with z, and how many labels it lays out. It reports
// and never gates. Item 102 moved the roads' tags out of the cached
// static layer and lays every label out each frame; the gap between
// the on and off rows is what that costs.
func BenchmarkDrawAFrame(b *testing.B) {
	for _, view := range []struct {
		name  string
		wheel float64
	}{{"fitted", 0}, {"close", 4}} {
		for _, labels := range []string{"on", "off"} {
			b.Run("metropolis/"+view.name+"/labels-"+labels, func(b *testing.B) {
				d, err := render.NewDrive(render.DriveOptions{Home: stageDemo(b, "metropolis"), Width: 1600, Height: 900})
				require.NoError(b, err)
				require.NoError(b, d.Run(settleTicks))
				if view.wheel > 0 {
					require.NoError(b, d.WheelAt(city.Point{X: 640, Y: 450}, view.wheel))
					require.NoError(b, d.Run(3*settleTicks))
				}
				if labels == "off" {
					require.NoError(b, d.Press("z"))
					require.NoError(b, d.Run(1))
				}
				for b.Loop() {
					require.NoError(b, d.Tick())
				}
				b.ReportMetric(float64(len(d.Shown().Labels)), "labels/frame")
			})
		}
	}
}
