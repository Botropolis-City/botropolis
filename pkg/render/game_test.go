package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 183: the top-down map is gone, and TopDown is Projection's zero
// value, so a game holds its scene isometric whatever it was handed.
func TestNewGame(t *testing.T) {
	t.Run("when the scene's projection was never set", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		NewGame(s, &recordingActor{}, ui.NewTheme(1), nil, nil)

		t.Run("it should draw the scene isometric", func(t *testing.T) {
			assert.Equal(t, city.Isometric, s.Projection())
		})
	})
}
