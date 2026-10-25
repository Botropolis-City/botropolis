//go:build !devtools

package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

func TestCityFlagsAbsentFromReleaseBuilds(t *testing.T) {
	t.Run("when city's help is printed in a build without devtools", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("city", "--help"))
		help := h.out.String()

		for _, flag := range []string{"screenshot", "keys", "hover", "record", "fps", "seconds", "script", "headless", "window", "signage", "anchors"} {
			t.Run("it should not list --"+flag, func(t *testing.T) {
				assert.NotContains(t, help, "--"+flag)
			})
		}
	})

	t.Run("when the city is opened in a build without devtools", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("city"))

		t.Run("it should ask for no screenshot", func(t *testing.T) {
			assert.Empty(t, h.city.screenshot)
		})
	})
}
