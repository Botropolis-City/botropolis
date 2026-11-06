package ui_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/ui"
)

func TestNotice(t *testing.T) {
	t0 := time.Date(2026, time.September, 18, 20, 0, 0, 0, time.UTC)
	fixed := ui.Message{Text: "needs-you: Fix the CI queue"}

	t.Run("when a notice is set", func(t *testing.T) {
		var n ui.Notice
		n.Set(fixed, t0)

		t.Run("it should read while it is fresh", func(t *testing.T) {
			assert.Equal(t, "needs-you: Fix the CI queue", n.Text(t0.Add(ui.NoticeFor-time.Millisecond)))
		})

		t.Run("it should be gone once it has expired", func(t *testing.T) {
			assert.Empty(t, n.Text(t0.Add(ui.NoticeFor)))
		})

		t.Run("and it is cleared early", func(t *testing.T) {
			n.Clear()

			t.Run("it should be gone at once", func(t *testing.T) {
				assert.Empty(t, n.Text(t0))
			})
		})
	})

	t.Run("when a notice is set with a tone and a key", func(t *testing.T) {
		var n ui.Notice
		m := ui.Message{Text: "packed up Sea conditions · undo", Tone: ui.ToneNeedsYou, Key: "u", SessionID: "sea"}
		n.Set(m, t0)
		current, ok := n.Current(t0)

		t.Run("it should return them with the text while fresh", func(t *testing.T) {
			assert.True(t, ok && current == m, "%v %v", ok, current)
		})

		t.Run("it should return nothing once it has expired", func(t *testing.T) {
			_, ok := n.Current(t0.Add(ui.NoticeFor))
			assert.False(t, ok)
		})
	})

	t.Run("when a second notice is set while the first is fresh", func(t *testing.T) {
		var n ui.Notice
		n.Set(fixed, t0)
		n.Set(ui.Message{Text: "saved"}, t0.Add(time.Second))

		t.Run("it should show only the second", func(t *testing.T) {
			assert.Equal(t, "saved", n.Text(t0.Add(time.Second)))
		})
	})

	t.Run("when a notice is set to nothing", func(t *testing.T) {
		var n ui.Notice
		n.Set(ui.Message{Text: "saved"}, t0)
		n.Set(ui.Message{}, t0)

		t.Run("it should read as nothing", func(t *testing.T) {
			assert.Empty(t, n.Text(t0))
		})
	})

	t.Run("when nothing was ever set", func(t *testing.T) {
		var n ui.Notice

		t.Run("it should read as nothing", func(t *testing.T) {
			_, ok := n.Current(t0)
			assert.False(t, ok)
		})
	})
}
