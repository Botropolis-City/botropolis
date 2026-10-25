package demo_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/demo"
)

func loadScenario(t *testing.T, name string) demo.Scenario {
	t.Helper()
	s, err := demo.LoadScenario(filepath.Join(scenariosDir, name+".yaml"))
	require.NoError(t, err)
	return s
}

func stills(s demo.Scenario, keep func(demo.Shot) bool) []string {
	var out []string
	for _, shot := range s.Shots {
		if shot.Record == nil && keep(shot) {
			out = append(out, shot.Name)
		}
	}
	return out
}

func hidesInterface(shot demo.Shot) bool { return slices.Contains(shot.Keys, "h") }

// Item 76: the website's stand-ins each wait on a still the film makes.
// A view added to the city without a still fails here, not on the site.
func TestTheSiteHasItsStills(t *testing.T) {
	layers := loadScenario(t, "layers")

	for _, v := range city.Views {
		t.Run("when layers is loaded", func(t *testing.T) {
			t.Run("it should have a still of the "+v.Name()+" view", func(t *testing.T) {
				assert.NotEmpty(t, stills(layers, func(s demo.Shot) bool { return s.View == v.Name() }))
			})
		})
	}

	t.Run("when layers is loaded", func(t *testing.T) {
		t.Run("it should have a plain day: a still with no view, its interface hidden", func(t *testing.T) {
			assert.NotEmpty(t, stills(layers, func(s demo.Shot) bool {
				return s.View == "" && s.Clock == "" && hidesInterface(s)
			}))
		})

		t.Run("it should have its night still", func(t *testing.T) {
			assert.NotEmpty(t, stills(layers, func(s demo.Shot) bool { return s.Name == "night" }))
		})
	})

	t.Run("when the metropolis is loaded", func(t *testing.T) {
		metropolis := loadScenario(t, "metropolis")

		t.Run("it should have a 1200x630 social card with its interface hidden", func(t *testing.T) {
			assert.NotEmpty(t, stills(metropolis, func(s demo.Shot) bool {
				return s.Window == "1200x630" && hidesInterface(s)
			}))
		})
	})
}
