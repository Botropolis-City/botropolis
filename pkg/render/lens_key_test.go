package render

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 177: a demo still is titled for the lens its key selects, so the
// scenario test reads the game's own table rather than a copy of it.
func TestLensOfKey(t *testing.T) {
	for i, want := range city.Lenses {
		name := fmt.Sprintf("Digit%d", i+1)
		t.Run("when a key name is "+name, func(t *testing.T) {
			got, ok := LensOfKey(name)

			t.Run("it should select "+want.Name(), func(t *testing.T) {
				assert.Equal(t, [2]any{want, true}, [2]any{got, ok})
			})
		})
	}

	for _, name := range []string{"Digit7", "Digit0", "V"} {
		t.Run("when a key name is "+name, func(t *testing.T) {
			_, ok := LensOfKey(name)

			t.Run("it should select no lens", func(t *testing.T) {
				assert.False(t, ok)
			})
		})
	}
}
