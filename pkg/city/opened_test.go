package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 159: until a snapshot opens the city, the window shows how far
// the first load has got instead of an empty island.
func TestSceneOpened(t *testing.T) {
	t.Run("when no snapshot has come", func(t *testing.T) {
		t.Run("it should not be open", func(t *testing.T) {
			assert.False(t, city.NewScene(city.NewLayout()).Opened())
		})
	})

	t.Run("when a snapshot has come", func(t *testing.T) {
		t.Run("it should be open", func(t *testing.T) {
			assert.True(t, scene(t).Opened())
		})
	})
}
