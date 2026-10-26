package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfirm(t *testing.T) {
	t0 := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)

	t.Run("when Escape asks", func(t *testing.T) {
		var c confirm
		c.ask(t0)

		t.Run("it should be armed", func(t *testing.T) {
			assert.True(t, c.armed(t0.Add(time.Second)))
		})

		t.Run("and y or enter answers in time", func(t *testing.T) {
			t.Run("it should confirm and stand down", func(t *testing.T) {
				assert.True(t, c.confirm(t0.Add(2*time.Second)))
				assert.False(t, c.armed(t0.Add(2*time.Second)))
			})
		})
	})

	t.Run("when Escape asks again while armed", func(t *testing.T) {
		var c confirm
		c.ask(t0)
		c.cancel()

		t.Run("it should stand down without confirming", func(t *testing.T) {
			assert.False(t, c.armed(t0))
			assert.False(t, c.confirm(t0))
		})
	})

	t.Run("when y or enter arrives with nothing asked", func(t *testing.T) {
		var c confirm

		t.Run("it should not confirm", func(t *testing.T) {
			assert.False(t, c.confirm(t0))
		})
	})

	t.Run("when the window runs out while the loop keeps updating", func(t *testing.T) {
		var c confirm
		c.ask(t0)
		for range confirmUpdates {
			c.tick()
		}

		t.Run("it should stand down on its own", func(t *testing.T) {
			assert.False(t, c.armed(t0.Add(ConfirmWithin+time.Millisecond)))
			assert.False(t, c.confirm(t0.Add(ConfirmWithin+time.Millisecond)))
		})
	})

	// Item 270: a city rebuilding a snapshot of 900 districts held its
	// loop for longer than the window, so the y pressed at once was read
	// after the prompt had stood down, and the city would not quit.
	t.Run("when q asks and the loop stalls past the window before y is read", func(t *testing.T) {
		var c confirm
		c.ask(t0)
		c.tick()

		t.Run("it should still confirm", func(t *testing.T) {
			assert.True(t, c.confirm(t0.Add(ConfirmWithin+time.Minute)))
		})
	})
}
