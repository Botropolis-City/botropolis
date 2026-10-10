package ui_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/ui"
)

func TestNotice(t *testing.T) {
	t0 := time.Date(2026, time.September, 18, 20, 0, 0, 0, time.UTC)

	t.Run("when a notice is set", func(t *testing.T) {
		var n ui.Notice
		n.Set("needs-you: Fix the CI queue", t0)

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

	t.Run("when a notice is set to nothing", func(t *testing.T) {
		var n ui.Notice
		n.Set("saved", t0)
		n.Set("", t0)

		t.Run("it should read as nothing", func(t *testing.T) {
			assert.Empty(t, n.Text(t0))
		})
	})

	t.Run("when nothing was ever set", func(t *testing.T) {
		var n ui.Notice

		t.Run("it should read as nothing", func(t *testing.T) {
			assert.Empty(t, n.Text(t0))
		})
	})
}
