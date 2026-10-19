package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 145: claude rm deletes the background job and its worktree, and
// leaves the conversation in Claude Code's files.
func TestDemolishText(t *testing.T) {
	t.Run("when the demolish action is described", func(t *testing.T) {
		t.Run("it should say the conversation stays", func(t *testing.T) {
			assert.Equal(t, "Deletes this session and its worktree, and takes it off the map. Its conversation stays in Claude Code's files.", city.ActionDemolish.Describe())
		})
	})
}
