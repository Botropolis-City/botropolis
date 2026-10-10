package version_test

import (
	"bytes"
	"testing"

	"botropolis.city/botropolis/pkg/version"
	"github.com/stretchr/testify/assert"
)

func TestPrint(t *testing.T) {
	t.Run("when Version has not been set by the linker", func(t *testing.T) {
		var out bytes.Buffer
		version.Print(&out, "botropolis")

		t.Run("it should print the binary name followed by dev", func(t *testing.T) {
			assert.Equal(t, "botropolis dev\n", out.String())
		})
	})
}
