package commands_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 90: muting a word is a correction kept with the groups.
func TestActorMute(t *testing.T) {
	t.Run("when a mute is acted on", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{Muted: []string{"headers"}}))
		require.NoError(t, commands.Actor{GroupStore: path}.Do(city.Action{Kind: city.ActionMute, Text: "kiln"}))
		memory, err := group.Load(path)
		require.NoError(t, err)

		t.Run("groups.json should hold the word, beside what it held", func(t *testing.T) {
			assert.Equal(t, []string{"headers", "kiln"}, memory.Muted)
		})
	})

	t.Run("when there is no group store, as for a run pointed at another home", func(t *testing.T) {
		err := commands.Actor{}.Do(city.Action{Kind: city.ActionMute, Text: "kiln"})

		t.Run("it should refuse, with the reason", func(t *testing.T) {
			assert.ErrorContains(t, err, "groups are kept only for your own home")
		})
	})
}
