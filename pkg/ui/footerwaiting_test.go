package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

// The needs-you line: the session that has waited longest, named in the
// footer above the key row, which a click attaches. Item 91.
func TestFooterWithWaiting(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{"drag", "pan"}, {"tab", "next needs-you"}}

	t.Run("when a session waits and there is no status", func(t *testing.T) {
		plain := ui.LayoutFooter(th, 800, 600, "", keys, measure7)
		f := plain.WithWaiting(th, 800, "needs-you: Sea conditions", measure7)

		t.Run("it should show the line", func(t *testing.T) {
			assert.Equal(t, "needs-you: Sea conditions", f.Waiting.Text)
		})

		t.Run("it should sit the label inside a plate, inset as a button's is", func(t *testing.T) {
			assert.Equal(t, [2]float64{f.WaitingRect.Min.X + th.Grid(), f.WaitingRect.Min.Y + th.Grid()/2}, [2]float64{f.Waiting.At.X, f.Waiting.At.Y})
		})

		t.Run("it should put the line above the key row", func(t *testing.T) {
			assert.LessOrEqual(t, f.WaitingRect.Max.Y, f.KeyRow.Min.Y)
		})

		t.Run("it should grow the footer to hold it", func(t *testing.T) {
			assert.LessOrEqual(t, f.Rect.Min.Y, f.WaitingRect.Min.Y)
		})

		t.Run("it should leave the key row where it was", func(t *testing.T) {
			assert.Equal(t, plain.KeyRow, f.KeyRow)
		})
	})

	t.Run("when a session waits and a status is shown", func(t *testing.T) {
		f := ui.LayoutFooter(th, 800, 600, "attached: Fix the CI queue", keys, measure7).WithWaiting(th, 800, "needs-you: Sea conditions", measure7)
		require.Len(t, f.Lines, 1)

		t.Run("it should draw the status above the line, clear of it", func(t *testing.T) {
			assert.LessOrEqual(t, f.Lines[0].At.Y+16, f.WaitingRect.Min.Y)
		})

		t.Run("it should keep the status inside the footer", func(t *testing.T) {
			assert.LessOrEqual(t, f.Rect.Min.Y, f.Lines[0].At.Y)
		})
	})

	t.Run("when nothing waits", func(t *testing.T) {
		plain := ui.LayoutFooter(th, 800, 600, "", keys, measure7)
		f := plain.WithWaiting(th, 800, "", measure7)

		t.Run("it should leave the footer as it was", func(t *testing.T) {
			assert.Equal(t, plain, f)
		})
	})

	t.Run("when the line is wider than the window", func(t *testing.T) {
		f := ui.LayoutFooter(th, 300, 600, "", keys, measure7).WithWaiting(th, 300, "needs-you: "+strings.Repeat("long ", 40), measure7)

		t.Run("it should fit the line inside the window", func(t *testing.T) {
			assert.LessOrEqual(t, f.WaitingRect.Max.X, 300.0)
		})
	})
}
