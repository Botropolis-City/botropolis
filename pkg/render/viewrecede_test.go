package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// A view is subtractive, so anything it has nothing to say about has to
// recede — including the objects that carry a tint for their own
// reasons. Those are the ones that slipped through, because the recede
// happens on the nil-tint path and a self-tinted sprite never reaches it.
func TestTowerTint(t *testing.T) {
	busy := &city.Tower{Server: state.Server{Name: "playwright", Calls: 12}}
	quiet := &city.Tower{Server: state.Server{Name: "atlassian"}}

	t.Run("when no view is up", func(t *testing.T) {
		t.Run("it should tint a busy tower for itself", func(t *testing.T) {
			require.NotNil(t, towerTint(busy, false, false))
		})

		t.Run("it should tint a quiet one differently", func(t *testing.T) {
			assert.NotEqual(t, towerTint(busy, false, false).R(), towerTint(quiet, false, false).R())
		})
	})

	t.Run("when a view is up", func(t *testing.T) {
		t.Run("it should give a busy tower no tint of its own, so the view can drain it", func(t *testing.T) {
			assert.Nil(t, towerTint(busy, true, false))
		})

		t.Run("it should do the same for a quiet one", func(t *testing.T) {
			assert.Nil(t, towerTint(quiet, true, false))
		})
	})
}

func TestTowerIsDimmedWhenUnlit(t *testing.T) {
	busy := &city.Tower{Server: state.Server{Name: "playwright", Calls: 12}}

	t.Run("when the contextual highlight is up and this tower is not the answer", func(t *testing.T) {
		t.Run("it should fade rather than keep its own colour", func(t *testing.T) {
			assert.Equal(t, dimmed().R(), towerTint(busy, false, true).R())
		})

		t.Run("it should fade inside a view too, where it would otherwise only recede", func(t *testing.T) {
			assert.Equal(t, dimmed().A(), towerTint(busy, true, true).A())
		})
	})
}
