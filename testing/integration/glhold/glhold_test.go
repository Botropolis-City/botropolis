//go:build linux

package glhold_test

import (
	"testing"

	"github.com/ebitengine/purego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/glhold"
)

const rtldNoload = 0x4

func loaded(name string) bool {
	h, err := purego.Dlopen(name, purego.RTLD_NOW|rtldNoload)
	if err != nil || h == 0 {
		return false
	}
	_ = purego.Dlclose(h)
	return true
}

func TestHoldKeepsLibraryMapped(t *testing.T) {
	t.Run("when a library is loaded and then held", func(t *testing.T) {
		const lib = "libm.so.6"
		require.False(t, loaded(lib), "%s must not be loaded before the test", lib)
		h, err := purego.Dlopen(lib, purego.RTLD_NOW)
		require.NoError(t, err)
		glhold.Hold(lib)

		t.Run("and its last handle is closed", func(t *testing.T) {
			require.NoError(t, purego.Dlclose(h))

			t.Run("it should still be mapped", func(t *testing.T) {
				assert.True(t, loaded(lib))
			})
		})
	})

	t.Run("when a library is not loaded and is held", func(t *testing.T) {
		const lib = "libBrokenLocale.so.1"
		require.False(t, loaded(lib), "%s must not be loaded before the test", lib)
		glhold.Hold(lib)

		t.Run("it should stay unloaded", func(t *testing.T) {
			assert.False(t, loaded(lib))
		})
	})
}
