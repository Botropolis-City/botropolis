package render

import (
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
)

// startLight is the scene's light for the light key's value. Item 308.
func startLight(value string) city.Light {
	switch value {
	case config.LightNight:
		return city.LightNight
	case config.LightDay:
		return city.LightDay
	}
	return city.LightAuto
}
