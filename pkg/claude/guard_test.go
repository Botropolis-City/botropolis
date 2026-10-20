package claude_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 165: the writers under the default state directory check it
// before they make anything there, so a test that has not isolated it
// fails instead of writing into the developer's own.
func TestWritersCheckTheStateDirectory(t *testing.T) {
	writers := []struct {
		name  string
		write func(bin string)
	}{
		{"a usage probe", func(bin string) { claude.RefreshUtilization(context.Background(), bin) }},
		{"a summary", func(bin string) { _, _ = claude.Summarise(context.Background(), bin, "User: hello") }},
	}
	for _, w := range writers {
		t.Run("when "+w.name+" runs without the state directory isolated", func(t *testing.T) {
			missing := filepath.Join(t.TempDir(), "no-such-claude")

			t.Run("it should panic before writing", func(t *testing.T) {
				assert.Panics(t, func() { w.write(missing) })
			})
		})
	}
}
