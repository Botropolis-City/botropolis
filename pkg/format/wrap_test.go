package format_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/format"
	"github.com/stretchr/testify/assert"
)

func TestWrap(t *testing.T) {
	t.Run("when the text fits", func(t *testing.T) {
		t.Run("it should stay on one line", func(t *testing.T) {
			assert.Equal(t, []string{"75 sessions | q to quit"}, format.Wrap("75 sessions | q to quit", 40))
		})
	})

	t.Run("when the text is too long", func(t *testing.T) {
		lines := format.Wrap("75 sessions | drag to pan | wheel to zoom | q to quit", 24)

		t.Run("it should break on spaces without exceeding the width", func(t *testing.T) {
			assert.Equal(t, []string{"75 sessions | drag to", "pan | wheel to zoom | q", "to quit"}, lines)
			for _, l := range lines {
				assert.LessOrEqual(t, len(l), 24)
			}
		})
	})

	t.Run("when a single word is wider than the width", func(t *testing.T) {
		t.Run("it should keep it whole", func(t *testing.T) {
			assert.Equal(t, []string{"supercalifragilistic"}, format.Wrap("supercalifragilistic", 5))
		})
	})

	t.Run("when the width is zero", func(t *testing.T) {
		t.Run("it should not wrap", func(t *testing.T) {
			assert.Equal(t, []string{"a b c"}, format.Wrap("a b c", 0))
		})
	})

	t.Run("when the text is empty", func(t *testing.T) {
		t.Run("it should give one empty line", func(t *testing.T) {
			assert.Equal(t, []string{""}, format.Wrap("", 10))
		})
	})
}

func TestClip(t *testing.T) {
	for _, c := range []struct {
		in    string
		width int
		want  string
	}{{"hello", 10, "hello"}, {"hello world", 5, "hell…"}, {"hello", 0, "hello"}, {"hi", 1, "…"}} {
		t.Run("when clipping "+c.in, func(t *testing.T) {
			assert.Equal(t, c.want, format.Clip(c.in, c.width))
		})
	}
}
