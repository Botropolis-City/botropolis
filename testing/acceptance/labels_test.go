package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

func labelKinds(labels []render.ShownLabel) map[string]int {
	kinds := map[string]int{}
	for _, l := range labels {
		kinds[l.Kind]++
	}
	return kinds
}

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
			kinds := labelKinds(labels)
			assert.Equal(t, []bool{true, true, true}, []bool{kinds["sign"] > 0, kinds["station label"] > 0, kinds["badge"] > 0})
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

// Item 102: z turns every label off, and the district signs on their
// posts stay, because they are part of the map.
func TestLabelsHotkey(t *testing.T) {
	home := stageDemo(t, "workday")

	t.Run("when the workday demo is fitted at 1600 by 900, and z is pressed", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: 1600, Height: 900})
		require.NoError(t, err)
		require.NoError(t, d.Run(settleTicks))
		before := labelKinds(d.Shown().Labels)
		require.Positive(t, before["badge"], "no badges to hide: the checks below pass on nothing")
		require.Positive(t, before["sign"], "no signs to keep: the checks below pass on nothing")
		require.NoError(t, d.Press("z"))
		require.NoError(t, d.Run(1))
		after := labelKinds(d.Shown().Labels)

		t.Run("it should draw no badge, station label or plate", func(t *testing.T) {
			assert.Equal(t, []int{0, 0, 0}, []int{after["badge"], after["station label"], after["plate"]})
		})

		t.Run("it should keep every district sign on its post", func(t *testing.T) {
			assert.Equal(t, before["sign"], after["sign"])
		})

		t.Run("and z is pressed again", func(t *testing.T) {
			require.NoError(t, d.Press("z"))
			require.NoError(t, d.Run(1))

			t.Run("it should draw the badges again", func(t *testing.T) {
				assert.Equal(t, before["badge"], labelKinds(d.Shown().Labels)["badge"])
			})
		})
	})
}
