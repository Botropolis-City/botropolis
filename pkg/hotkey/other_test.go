//go:build !darwin

package hotkey

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenMacOSElsewhere(t *testing.T) {
	t.Run("when the desktop is macOS and this build is not for macOS", func(t *testing.T) {
		_, err := Open("darwin", func(string) string { return "" })

		t.Run("it should say this desktop has no way to register one", func(t *testing.T) {
			assert.True(t, errors.Is(err, ErrUnsupported))
		})
	})
}
