package demo

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// pressLensKeys plays lens keys into a fresh scene the way the city's
// number keys do: DigitN selects the Nth lens.
func pressLensKeys(t *testing.T, keys []string) city.View {
	t.Helper()
	s := city.NewScene(city.NewLayout())
	for _, k := range keys {
		n, err := strconv.Atoi(strings.TrimPrefix(k, "Digit"))
		require.NoError(t, err, "key %q", k)
		require.True(t, n >= 1 && n <= len(city.Lenses), "key %q names no lens", k)
		s.SelectLens(city.Lenses[n-1])
	}
	return s.View()
}

// Item 76: one still per view, reached by the same keys a person
// presses, so the stills cannot drift from the app's lenses.
func TestViewKeys(t *testing.T) {
	for _, v := range city.Views {
		t.Run("when a shot asks for the "+v.Name()+" view", func(t *testing.T) {
			keys, err := viewKeys(v.Name())
			require.NoError(t, err)

			t.Run("it should press keys that bring a fresh scene to that view", func(t *testing.T) {
				assert.Equal(t, v, pressLensKeys(t, keys))
			})
		})
	}

	t.Run("when a shot asks for a view the city does not have", func(t *testing.T) {
		err := Scenario{Sessions: []Placement{{Project: "beacon", State: "empty"}},
			Shots: []Shot{{Name: "a", View: "weather"}}}.Validate()

		t.Run("it should refuse the scenario, naming the view", func(t *testing.T) {
			assert.ErrorContains(t, err, `"weather"`)
		})
	})
}

func TestShotMedia(t *testing.T) {
	scenario := Scenario{Name: "layers"}

	t.Run("when a still asks for a view", func(t *testing.T) {
		m := shotMedia(scenario, Shot{Name: "spend", View: "spend"})

		t.Run("it should name the view in its manifest entry", func(t *testing.T) {
			assert.Equal(t, "spend", m.View)
		})
	})

	t.Run("when a still asks for none", func(t *testing.T) {
		m := shotMedia(scenario, Shot{Name: "day"})

		t.Run("it should leave the view out", func(t *testing.T) {
			assert.Empty(t, m.View)
		})
	})
}
