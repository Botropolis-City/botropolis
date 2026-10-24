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

func TestModel(t *testing.T) {
	for _, c := range []struct {
		name, id, want string
	}{
		{"when the id is claude-<name>-<date>, it should drop both", "claude-haiku-4-5-20251001", "haiku-4-5"},
		{"when the id has no date, it should drop only the prefix", "claude-opus-4-7", "opus-4-7"},
		{"when the id is not Claude's, it should leave it alone", "gpt-x-20250101", "gpt-x-20250101"},
		{"when the id ends in a number that is not a date, it should keep it", "claude-tiny-1", "tiny-1"},
		{"when the id is empty, it should stay empty", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, format.Model(c.id))
		})
	}
}
