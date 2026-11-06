package state_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 348: a session names the groups it was taken out of by hand, so
// the city moves its building out of that neighbourhood once.
func TestLoaderGroupLeft(t *testing.T) {
	const sidC = "3333cccc-0000-0000-0000-000000000000"
	const webDir = "/home/avesta/workspaces/github/acme/web"
	home := groupedHome(t)
	home.Transcript(sidC, webDir,
		helpers.UserPrompt(sidC, webDir, "2026-09-16T16:00:00.000Z"),
		helpers.AssistantReply(sidC, "msg_03", "2026-09-16T16:00:10.000Z"))
	home.Prompt(sidC, webDir, "the PROJ-123 page", time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC))
	path := filepath.Join(t.TempDir(), "groups.json")
	loader := state.NewLoader(home.Path, dead).WithGroups(path)
	_, err := loader.Load(now)
	require.NoError(t, err)
	memory, err := group.Load(path)
	require.NoError(t, err)
	require.Len(t, memory.Groups, 1)
	id := memory.Groups[0].ID

	t.Run("when a session is taken out of a group that goes on without it", func(t *testing.T) {
		require.NoError(t, group.Save(path, memory.WithTakenOut(sidC, id)))
		snapshot, err := loader.Load(now.Add(time.Second))
		require.NoError(t, err)

		t.Run("it should name the group on the session", func(t *testing.T) {
			assert.Equal(t, []string{id}, sessionByID(t, snapshot, sidC).GroupLeft)
		})

		t.Run("it should name none on the members left in it", func(t *testing.T) {
			assert.Empty(t, append(sessionByID(t, snapshot, sidA).GroupLeft, sessionByID(t, snapshot, sidB).GroupLeft...))
		})
	})
}
