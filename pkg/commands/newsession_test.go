package commands_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorNewSession(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC) }
	started := func(t *testing.T, ctl *fakeController, action city.Action) (group.Memory, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "groups.json")
		actor := commands.Actor{Sessions: commands.NewSessions(t.TempDir(), ctl), GroupStore: path, Now: now}
		err := actor.Do(action)
		memory, loadErr := group.Load(path)
		require.NoError(t, loadErr)
		return memory, err
	}

	t.Run("when a session is started in a group", func(t *testing.T) {
		ctl := &fakeController{id: "0898d7e4"}
		memory, err := started(t, ctl, city.Action{Kind: city.ActionNewSession, Dir: "/w/api", Prompt: "bump it", Group: "g1"})
		require.NoError(t, err)

		t.Run("it should start claude in the background in that directory with the prompt", func(t *testing.T) {
			assert.Equal(t, []string{"new /w/api bump it"}, ctl.calls)
		})

		t.Run("it should record that its job was started from the group", func(t *testing.T) {
			assert.Equal(t, map[string]string{"0898d7e4": "g1"}, memory.StartedFrom)
		})
	})

	t.Run("when a session is started in a new group", func(t *testing.T) {
		ctl := &fakeController{id: "0898d7e4"}
		memory, err := started(t, ctl, city.Action{Kind: city.ActionNewSession, Dir: "/w/api", Prompt: "go", GroupName: "Gateway"})
		require.NoError(t, err)

		t.Run("it should store the group under the name typed, as a rename", func(t *testing.T) {
			require.Len(t, memory.Groups, 1)
			assert.Equal(t, group.Stored{ID: memory.StartedFrom["0898d7e4"], Name: "Gateway", Renamed: true}, memory.Groups[0])
		})
	})

	t.Run("when a session is started in no group", func(t *testing.T) {
		ctl := &fakeController{id: "0898d7e4"}
		memory, err := started(t, ctl, city.Action{Kind: city.ActionNewSession, Dir: "/w/api", Prompt: "go"})
		require.NoError(t, err)

		t.Run("it should record nothing", func(t *testing.T) {
			assert.Empty(t, memory.StartedFrom)
		})
	})

	t.Run("when claude --bg fails", func(t *testing.T) {
		ctl := &fakeController{failure: errors.New("not logged in")}
		memory, err := started(t, ctl, city.Action{Kind: city.ActionNewSession, Dir: "/w/api", Prompt: "go", Group: "g1"})

		t.Run("it should record no started-from", func(t *testing.T) {
			assert.Empty(t, memory.StartedFrom)
		})

		t.Run("it should say why", func(t *testing.T) {
			assert.ErrorContains(t, err, "not logged in")
		})
	})
}
