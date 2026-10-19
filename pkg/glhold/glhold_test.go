//go:build linux || freebsd

package glhold

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHold(t *testing.T) {
	t.Run("when the city holds the GL libraries", func(t *testing.T) {
		var names []string
		var modes []int
		hold(func(name string, mode int) (uintptr, error) {
			names = append(names, name)
			modes = append(modes, mode)
			return 1, nil
		}, GL)

		t.Run("it should ask for every EGL, GLES and GLX library", func(t *testing.T) {
			assert.Equal(t, []string{"libEGL.so.1", "libEGL_mesa.so.0", "libGLESv2.so.2", "libGL.so.1", "libGLX.so.0", "libGLX_mesa.so.0"}, names)
		})

		for _, flag := range []struct {
			name string
			bit  int
		}{
			{"it should open each without loading it", rtldNoload},
			{"it should open each so it cannot be unloaded", rtldNodelete},
		} {
			t.Run(flag.name, func(t *testing.T) {
				with := 0
				for _, m := range modes {
					if m&flag.bit != 0 {
						with++
					}
				}
				assert.Equal(t, len(GL), with)
			})
		}

		t.Run("and a library is not loaded", func(t *testing.T) {
			var names []string
			hold(func(name string, _ int) (uintptr, error) {
				names = append(names, name)
				if name == GL[0] {
					return 0, errors.New("not loaded")
				}
				return 1, nil
			}, GL)

			t.Run("it should carry on to the rest", func(t *testing.T) {
				assert.Equal(t, GL, names)
			})
		})
	})
}
