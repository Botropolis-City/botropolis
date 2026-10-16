package group_test

import (
	"fmt"
	"testing"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

// history is twenty unrelated sessions that all say "fix the test", so
// that wording is common and anything two sessions add to it is rare.
func history(extra map[string][]string) map[string][]string {
	docs := map[string][]string{}
	for i := 0; i < 20; i++ {
		docs[fmt.Sprintf("filler-%02d", i)] = []string{fmt.Sprintf("fix the test in module%d", i)}
	}
	for id, prompts := range extra {
		docs[id] = prompts
	}
	return docs
}

func TestWording(t *testing.T) {
	t.Run("when two sessions share words that are rare across the whole history", func(t *testing.T) {
		w := group.NewWording(history(map[string][]string{
			"a": {"add rate-limit headers to the limiter"},
			"b": {"read the rate-limit headers in the middleware"},
		}), nil)

		t.Run("it should count", func(t *testing.T) {
			assert.True(t, w.Related("a", "b"))
		})
	})

	t.Run("when they share only common words", func(t *testing.T) {
		w := group.NewWording(history(map[string][]string{
			"a": {"fix the test"},
			"b": {"fix the test"},
		}), nil)

		t.Run("it should not count", func(t *testing.T) {
			assert.False(t, w.Related("a", "b"))
		})
	})

	t.Run("when the shared words are only in a later prompt", func(t *testing.T) {
		w := group.NewWording(history(map[string][]string{
			"a": {"hi", "add rate-limit headers to the limiter"},
			"b": {"go", "ok", "read the rate-limit headers in the middleware"},
		}), nil)

		t.Run("it should still count", func(t *testing.T) {
			assert.True(t, w.Related("a", "b"))
		})
	})

	t.Run("when the shared words are muted", func(t *testing.T) {
		w := group.NewWording(history(map[string][]string{
			"a": {"add rate-limit headers to the limiter"},
			"b": {"read the rate-limit headers in the middleware"},
		}), []string{"rate-limit", "headers"})

		t.Run("it should not count", func(t *testing.T) {
			assert.False(t, w.Related("a", "b"))
		})
	})
}
