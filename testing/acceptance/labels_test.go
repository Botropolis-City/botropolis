package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// Item 102: in a fitted view of the workday demo, no group badge,
// station label or district sign covers another.
func TestLabelsDoNotOverlap(t *testing.T) {
	home := stageDemo(t, "workday")

	t.Run("when the workday demo is fitted at 1600 by 900", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: 1600, Height: 900})
		require.NoError(t, err)
		require.NoError(t, d.Run(settleTicks))
		labels := d.Shown().Labels

		t.Run("it should draw at least one of each kind, so the check cannot pass on nothing", func(t *testing.T) {
			kinds := map[string]bool{}
			for _, l := range labels {
				kinds[l.Kind] = true
			}
			assert.Equal(t, map[string]bool{"sign": true, "station label": true, "badge": true}, kinds)
		})

		t.Run("it should leave no two drawn labels overlapping", func(t *testing.T) {
			var overlaps []string
			for i := range labels {
				for j := i + 1; j < len(labels); j++ {
					if labels[i].Rect.Overlaps(labels[j].Rect) {
						overlaps = append(overlaps, labels[i].Kind+" on "+labels[j].Kind)
					}
				}
			}
			assert.Empty(t, overlaps)
		})
	})
}
