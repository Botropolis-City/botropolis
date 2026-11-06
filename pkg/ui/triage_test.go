package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutSnoozePicker(t *testing.T) {
	th := ui.NewTheme(1)
	choices := []string{"in an hour", "this evening at 18:00", "tomorrow at 09:00"}

	t.Run("when it opens", func(t *testing.T) {
		p := ui.LayoutSnoozePicker(th, 1200, 800, choices, 0, "", "", measure7)

		t.Run("it should offer each ready time and a line to type one", func(t *testing.T) {
			assert.Len(t, p.Rows, len(choices)+1)
		})

		t.Run("it should mark the choice under the cursor", func(t *testing.T) {
			assert.Equal(t, 0, p.Cursor)
		})
	})

	t.Run("when a typed time is refused", func(t *testing.T) {
		p := ui.LayoutSnoozePicker(th, 1200, 800, choices, 3, "07:00", "that time has passed", measure7)

		t.Run("it should say why under the line", func(t *testing.T) {
			assert.True(t, strings.Contains(p.Error.Text, "has passed") && p.Error.At.Y > p.Rows[3].At.Y)
		})
	})
}
