package city_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

// Bug 48. Aria: "I think I want cars on the main view not just on their
// own view."

func TestBaseViewTraffic(t *testing.T) {
	t.Run("when the city is on its base view", func(t *testing.T) {
		t.Run("it should drive the cars, which used to need their own view", func(t *testing.T) {
			assert.True(t, city.ViewAttention.Shows(city.NetworkTraffic))
		})

		t.Run("it should still leave the wires to the views that explain them", func(t *testing.T) {
			assert.False(t, city.ViewAttention.Shows(city.NetworkWires))
		})

		t.Run("it should still leave the beams to theirs", func(t *testing.T) {
			assert.False(t, city.ViewAttention.Shows(city.NetworkBeams))
		})
	})
}
