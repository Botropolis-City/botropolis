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

// fitted is a demo's city fitted at 1600 by 900 and settled.
func fitted(t *testing.T, demo string) *render.Drive {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: stageDemo(t, demo), Width: 1600, Height: 900})
	require.NoError(t, err)
	require.NoError(t, d.Run(settleTicks))
	return d
}

// Item 102: in a fitted view of a demo, no group badge, station label,
// district sign, first sighting's caption or road tag covers another.
func TestLabelsDoNotOverlap(t *testing.T) {
	for _, c := range []struct {
		demo  string
		kinds []string
	}{
		{"workday", []string{"sign", "station label", "badge", "caption"}},
		{"workshop", []string{"station label", "badge", "tag"}},
	} {
		t.Run("when the "+c.demo+" demo is fitted at 1600 by 900", func(t *testing.T) {
			labels := fitted(t, c.demo).Shown().Labels

			t.Run("it should lay out at least one of each kind it shows, so the check cannot pass on nothing", func(t *testing.T) {
				kinds := labelKinds(labels)
				var missing []string
				for _, k := range c.kinds {
					if kinds[k] == 0 {
						missing = append(missing, k)
					}
				}
				assert.Empty(t, missing)
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
}

// Item 102: z turns every label off, and the district signs on their
// posts stay, because they are part of the map.
func TestLabelsHotkey(t *testing.T) {
	t.Run("when the workday demo is fitted at 1600 by 900, and z is pressed", func(t *testing.T) {
		d := fitted(t, "workday")
		before := labelKinds(d.Shown().Labels)
		require.Positive(t, before["badge"], "no badges to hide: the checks below pass on nothing")
		require.Positive(t, before["sign"], "no signs to keep: the checks below pass on nothing")
		require.NoError(t, d.Press("z"))
		require.NoError(t, d.Run(1))
		after := labelKinds(d.Shown().Labels)

		t.Run("it should draw no badge, station label, plate, caption or tag", func(t *testing.T) {
			assert.Equal(t, []int{0, 0, 0, 0, 0}, []int{after["badge"], after["station label"], after["plate"], after["caption"], after["tag"]})
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
