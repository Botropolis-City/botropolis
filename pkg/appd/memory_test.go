package appd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryLimit(t *testing.T) {
	const firstLaunchBudget = 256 << 20

	t.Run("when GOMEMLIMIT is not set", func(t *testing.T) {
		limit := memoryLimit("")

		t.Run("it should not cap the heap below the 256 MiB first-launch budget", func(t *testing.T) {
			assert.GreaterOrEqual(t, limit, int64(firstLaunchBudget))
		})
	})

	t.Run("when GOMEMLIMIT is set", func(t *testing.T) {
		limit := memoryLimit("1GiB")

		t.Run("it should leave the limit to the runtime", func(t *testing.T) {
			assert.Negative(t, limit)
		})
	})
}
