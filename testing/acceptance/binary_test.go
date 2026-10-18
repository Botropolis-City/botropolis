package acceptance_test

import (
	"reflect"
	"testing"

	"botropolis.city/botropolis/pkg/app"
	"github.com/stretchr/testify/assert"
)

// The tests drive a binary that TestMain builds, which go test cannot see: it
// keys a cached result on the test binary alone. So the test binary has to
// link the code the built binary runs, and a blank import is not enough,
// because the linker drops whatever nothing reaches. Referencing app.Main,
// which is all cmd/botropolis calls, links what the binary links, so any
// change the binary would carry changes this binary too (item 117).
var entry = app.Main

func TestTheTestBinaryLinksTheBinary(t *testing.T) {
	t.Run("when the acceptance test binary is built", func(t *testing.T) {
		t.Run("it should link the entry point cmd/botropolis runs", func(t *testing.T) {
			assert.NotZero(t, reflect.ValueOf(entry).Pointer())
		})
	})
}
