package format_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/format"
	"github.com/stretchr/testify/assert"
)

func TestCount(t *testing.T) {
	for _, c := range []struct {
		name string
		n    int
		want string
	}{
		{"when there is one, it should use the singular", 1, "1 PR"},
		{"when there are two, it should use the plural", 2, "2 PRs"},
		{"when there are none, it should use the plural", 0, "0 PRs"},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, format.Count(c.n, "PR", "PRs"))
		})
	}
}

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
