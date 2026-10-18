package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/render"
)

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
