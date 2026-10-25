//go:build devtools

package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

func TestFrameFlags(t *testing.T) {
	t.Run("when asked for a screenshot", func(t *testing.T) {
		t.Run("and the city subcommand is used", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--screenshot", "out.png"))

			t.Run("it should hand the path to the city", func(t *testing.T) {
				assert.Equal(t, "out.png", h.city.screenshot)
			})
		})

		t.Run("and no subcommand is used", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("--screenshot", "out.png"))

			t.Run("it should hand the path to the city as well", func(t *testing.T) {
				assert.Equal(t, "out.png", h.city.screenshot)
			})
		})

		t.Run("and no path is given", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city"))

			t.Run("it should ask for no screenshot", func(t *testing.T) {
				assert.Empty(t, h.city.screenshot)
			})
		})

		t.Run("and a recording is asked for", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--record", "frames", "--seconds", "12"))

			t.Run("it should hand the directory and the length to the city", func(t *testing.T) {
				assert.Equal(t, "frames", h.city.record)
				assert.Equal(t, 12.0, h.city.seconds)
			})
		})

		// Item 76: where each named thing landed in the still.
		t.Run("and its anchors are asked for", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--screenshot", "out.png", "--anchors", "out.anchors.json"))

			t.Run("it should hand the city the path to write them to", func(t *testing.T) {
				assert.Equal(t, "out.anchors.json", h.city.anchors)
			})
		})

		t.Run("and no anchors are asked for", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--screenshot", "out.png"))

			t.Run("it should ask for none", func(t *testing.T) {
				assert.Empty(t, h.city.anchors)
			})
		})

		t.Run("and headless is asked for", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--headless", "--screenshot", "out.png"))

			t.Run("it should tell the city", func(t *testing.T) {
				assert.True(t, h.city.headless)
			})
		})

		t.Run("and keys are given", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--screenshot", "out.png", "--keys", "equal,equal,b"))

			t.Run("it should hand the keys to the city in order", func(t *testing.T) {
				assert.Equal(t, []string{"equal", "equal", "b"}, h.city.keys)
			})
		})
	})
}
