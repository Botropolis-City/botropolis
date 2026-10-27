package commands_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/group"
)

type countingController struct {
	*fakeController
	mu sync.Mutex
	n  int
}

func (c *countingController) New(context.Context, string, string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return fmt.Sprintf("%08x", c.n), nil
}

func TestActorGroupsWriters(t *testing.T) {
	t.Run("when moves are made while starts are writing the groups file", func(t *testing.T) {
		const each = 40
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{Groups: []group.Stored{{ID: "g1", Name: "Kiln"}}}))
		ctl := &countingController{fakeController: &fakeController{}}
		home := t.TempDir()
		var wg sync.WaitGroup
		for i := range each {
			wg.Add(2)
			go func() {
				defer wg.Done()
				actor := commands.Actor{Sessions: commands.NewSessions(home, ctl), GroupStore: path}
				_ = actor.Do(city.Action{Kind: city.ActionNewSession, Dir: "/w/kiln", Group: "g1"})
			}()
			go func() {
				defer wg.Done()
				actor := commands.Actor{Sessions: commands.NewSessions(home, ctl), GroupStore: path}
				_ = actor.Do(city.Action{Kind: city.ActionMove, SessionID: fmt.Sprintf("s%02d", i), Group: "g1"})
			}()
		}
		wg.Wait()
		memory, err := group.Load(path)
		require.NoError(t, err)

		t.Run("it should keep every start", func(t *testing.T) {
			assert.Len(t, memory.StartedFrom, each)
		})

		t.Run("it should keep every move", func(t *testing.T) {
			assert.Len(t, memory.Moves, each)
		})
	})
}
