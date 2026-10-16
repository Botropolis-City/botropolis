package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestField(t *testing.T) {
	t.Run("when characters are typed", func(t *testing.T) {
		f := field{text: []rune("ac"), caret: 1}
		f.insert("b")

		t.Run("it should insert them at the caret", func(t *testing.T) {
			assert.Equal(t, "abc", f.query())
		})
	})

	t.Run("when Backspace is pressed", func(t *testing.T) {
		f := field{text: []rune("héllo"), caret: 2}
		f.backspace()

		t.Run("it should delete the rune before the caret", func(t *testing.T) {
			assert.Equal(t, "hllo", f.query())
		})
	})

	t.Run("when an input method is composing", func(t *testing.T) {
		f := field{text: []rune("deploy "), caret: 7}
		f.compose("にほん")
		shown, _ := f.shown()

		t.Run("it should draw the composition without filtering on it", func(t *testing.T) {
			assert.Equal(t, [2]string{"deploy にほん", "deploy "}, [2]string{shown, f.query()})
		})
	})
}
