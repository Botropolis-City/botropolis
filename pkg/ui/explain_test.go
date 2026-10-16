package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutExplain(t *testing.T) {
	th := ui.NewTheme(1)
	window := city.RectAt(0, 0, 1200, 800)
	at := func(k city.Kind, x, y float64) ui.Seen {
		return ui.Seen{Kind: k, Rect: city.RectAt(x, y, 20, 20)}
	}

	t.Run("when several of one kind are visible", func(t *testing.T) {
		caps := ui.LayoutExplain(th, []ui.Seen{at(city.KindDrone, 100, 100), at(city.KindDrone, 600, 400), at(city.KindDrone, 900, 700)}, window, true, measure7)

		t.Run("it should caption one", func(t *testing.T) {
			assert.Len(t, caps, 1)
		})
	})

	t.Run("for every visible kind", func(t *testing.T) {
		var seen []ui.Seen
		for i, k := range city.Kinds {
			seen = append(seen, at(k, 60+float64(i%4)*280, 60+float64(i/4)*120))
		}
		caps := ui.LayoutExplain(th, seen, window, true, measure7)
		got := map[city.Kind]bool{}
		for _, c := range caps {
			got[c.Kind] = true
		}
		for _, k := range city.Kinds {
			t.Run("when it is "+k.Name()+", it should caption it", func(t *testing.T) {
				assert.True(t, got[k])
			})
		}
	})

	t.Run("when many are visible in one place", func(t *testing.T) {
		var seen []ui.Seen
		for _, k := range city.Kinds {
			seen = append(seen, at(k, 600, 400))
		}
		caps := ui.LayoutExplain(th, seen, window, true, measure7)
		require.NotEmpty(t, caps)

		t.Run("it should never overlap two captions", func(t *testing.T) {
			for i := range caps {
				for j := i + 1; j < len(caps); j++ {
					require.False(t, caps[i].Plate.Rect.Overlaps(caps[j].Plate.Rect), "%s and %s", caps[i].Text, caps[j].Text)
				}
			}
		})
	})

	t.Run("when something is at the window's edge", func(t *testing.T) {
		caps := ui.LayoutExplain(th, []ui.Seen{at(city.KindStation, 1190, 0)}, window, true, measure7)
		require.Len(t, caps, 1)

		t.Run("it should keep the caption inside the window", func(t *testing.T) {
			r := caps[0].Plate.Rect
			assert.True(t, window.Contains(r.Min) && window.Contains(r.Max))
		})
	})

	t.Run("when the key is released", func(t *testing.T) {
		caps := ui.LayoutExplain(th, []ui.Seen{at(city.KindDrone, 100, 100)}, window, false, measure7)

		t.Run("it should show no captions", func(t *testing.T) {
			assert.Empty(t, caps)
		})
	})
}
