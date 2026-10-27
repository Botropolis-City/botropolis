package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
)

// Item 308: the light key's value is the scene's light at start.
func TestStartLight(t *testing.T) {
	for value, want := range map[string]city.Light{
		config.LightLive:  city.LightAuto,
		config.LightNight: city.LightNight,
		config.LightDay:   city.LightDay,
		"":                city.LightAuto,
	} {
		t.Run("when light is "+value+", it should start "+want.String(), func(t *testing.T) {
			assert.Equal(t, want, startLight(value))
		})
	}
}
