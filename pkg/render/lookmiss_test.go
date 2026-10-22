package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 202: a look that misses says which of its key's fields changed
// since its run's last frame, so a settled frame's work names its cause.
func TestLookMissChanged(t *testing.T) {
	was := lookKey{id: "a", title: "before", zoom: 1}

	t.Run("when a run's key changes only its title", func(t *testing.T) {
		now := was
		now.title = "after"

		t.Run("it should name title", func(t *testing.T) {
			assert.Equal(t, "title", lookChanged(was, true, now))
		})
	})

	t.Run("when a run's key changes its title and its zoom", func(t *testing.T) {
		now := was
		now.title, now.zoom = "after", 2

		t.Run("it should name both, in the key's order", func(t *testing.T) {
			assert.Equal(t, "zoom,title", lookChanged(was, true, now))
		})
	})

	t.Run("when a run has no previous key", func(t *testing.T) {
		t.Run("it should say new", func(t *testing.T) {
			assert.Equal(t, "new", lookChanged(lookKey{}, false, was))
		})
	})

	t.Run("when a run's key is the one it had", func(t *testing.T) {
		t.Run("it should say unchanged", func(t *testing.T) {
			assert.Equal(t, "unchanged", lookChanged(was, true, was))
		})
	})
}
