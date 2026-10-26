package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 270: a city of 900 districts settled at 562 MiB, a 342 MB heap
// for 170 MB live, with the collector waiting for the heap to double.
// Aria set the city a soft limit, as the daemon has (item 157).
func TestCityMemoryLimit(t *testing.T) {
	t.Run("when GOMEMLIMIT is not set", func(t *testing.T) {
		limit := cityMemoryLimit("")

		t.Run("it should limit Go's heap to 300 MiB", func(t *testing.T) {
			assert.Equal(t, int64(300<<20), limit)
		})
	})

	t.Run("when GOMEMLIMIT is set", func(t *testing.T) {
		limit := cityMemoryLimit("1GiB")

		t.Run("it should leave the limit to the runtime", func(t *testing.T) {
			assert.Negative(t, limit)
		})
	})
}
