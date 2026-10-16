package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestReference(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{Key: "tab", Action: "next needs-you"}, {Key: "v", Action: "lenses"}}
	ref := ui.NewReference(keys)

	t.Run("it should have an entry for every kind", func(t *testing.T) {
		for _, k := range city.Kinds {
			_, ok := ref.Entry(k.Name())
			require.True(t, ok, k.Name())
		}
	})

	t.Run("it should have an entry for the keys", func(t *testing.T) {
		e, ok := ref.Entry(ui.KeysEntry)
		require.True(t, ok)
		assert.Contains(t, e.Lines, "tab — next needs-you")
	})

	t.Run("when a term is clicked", func(t *testing.T) {
		ref := ui.NewReference(keys)
		ref.Open(city.KindSession.Name())
		laid := ui.LayoutReference(th, 1200, 800, ref, measure7)
		var drone ui.Button
		for _, b := range laid.Terms {
			if b.Label.Text == city.KindDrone.Name() {
				drone = b
			}
		}
		require.NotZero(t, drone.Rect.Area(), "the session entry should link to drone")
		term, ok := laid.Hit(drone.Rect.Center())
		require.True(t, ok)
		ref.Open(term)

		t.Run("it should open that term's entry", func(t *testing.T) {
			assert.Equal(t, city.KindDrone.Name(), ref.Current().ID)
		})

		t.Run("and back is pressed", func(t *testing.T) {
			ref.Back()

			t.Run("it should return to the entry before", func(t *testing.T) {
				assert.Equal(t, city.KindSession.Name(), ref.Current().ID)
			})
		})
	})
}
