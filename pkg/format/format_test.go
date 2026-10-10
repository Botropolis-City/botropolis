package format_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/format"
	"github.com/stretchr/testify/assert"
)

func TestUSD(t *testing.T) {
	t.Run("when the cost is known", func(t *testing.T) {
		t.Run("it should show it as an estimate to the cent", func(t *testing.T) {
			assert.Equal(t, "~$12.50", format.USD(12.5, true))
		})
	})

	t.Run("when the cost is unknown", func(t *testing.T) {
		t.Run("it should show a dash, never a number", func(t *testing.T) {
			assert.Equal(t, "\u2014", format.USD(0, false))
		})
	})
}
