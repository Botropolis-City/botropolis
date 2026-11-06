package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 348: a frame can be shot with a group focused, its buildings lit
// and the rest of the city receded, from where the camera already
// stands, so it compares with the same frame unfocused.
func TestFrameFocus(t *testing.T) {
	t.Run("when a frame asks for a group's focus", func(t *testing.T) {
		rc := newRecordCity(t)
		before := *rc.scene.Camera()
		rc.g.focusFor = "g1"
		rc.g.applyFrameFocus()

		t.Run("it should focus the group", func(t *testing.T) {
			assert.Equal(t, "g1", rc.scene.FocusedGroup())
		})

		t.Run("it should leave the camera where it stood", func(t *testing.T) {
			assert.Equal(t, before, *rc.scene.Camera())
		})

		t.Run("and the frame after it is worked out", func(t *testing.T) {
			rc.scene.ClearFocus()
			rc.g.applyFrameFocus()

			t.Run("it should not focus it again", func(t *testing.T) {
				assert.Empty(t, rc.scene.FocusedGroup())
			})
		})
	})

	t.Run("when a frame asks for no focus", func(t *testing.T) {
		rc := newRecordCity(t)
		rc.g.applyFrameFocus()

		t.Run("it should focus nothing", func(t *testing.T) {
			assert.Empty(t, rc.scene.FocusedGroup())
		})
	})
}
