package packaging_test

import (
	"reflect"
	"testing"

	"botropolis.city/botropolis/pkg/app"
	"github.com/stretchr/testify/assert"
)

// TestCompletionScripts drives a binary it builds, which go test cannot see.
// Referencing app.Main links the code that binary runs into this test binary,
// so the cache keys on it (item 117; see testing/acceptance/binary_test.go).
var entry = app.Main

func TestTheTestBinaryLinksTheBinary(t *testing.T) {
	t.Run("when the packaging test binary is built", func(t *testing.T) {
		t.Run("it should link the entry point cmd/botropolis runs", func(t *testing.T) {
			assert.NotZero(t, reflect.ValueOf(entry).Pointer())
		})
	})
}
