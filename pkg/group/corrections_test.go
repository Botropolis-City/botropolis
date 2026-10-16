package group_test

import (
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorrections(t *testing.T) {
	members := []group.Member{
		member("a", "/w/lib", 0, "PROJ-1 start"),
		member("b", "/w/api", 3*time.Hour, "PROJ-1 carry on"),
		member("c", "/w/logs", 6*time.Hour, "LOG-4 buffer"),
		member("d", "/w/config", 9*time.Hour, "LOG-4 schema"),
	}
	stored := []group.Stored{{ID: "g-proj", Members: []string{"a", "b"}}, {ID: "g-log", Members: []string{"c", "d"}}}

	t.Run("when a session is moved to another group", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored, Moves: []group.Move{{Session: "b", Group: "g-log"}}}, members...)
		g, _ := result.GroupOf("b")

		t.Run("it should join that group", func(t *testing.T) {
			assert.Equal(t, "g-log", g.ID)
		})

		t.Run("it should be sure", func(t *testing.T) {
			assert.Equal(t, group.Sure, g.Member("b").Tier)
		})

		t.Run("it should say it was moved", func(t *testing.T) {
			assert.Equal(t, group.SignalMoved, g.Member("b").Reason.Signal)
		})

		t.Run("it should leave the group it was detected in", func(t *testing.T) {
			proj, _ := result.GroupOf("a")
			assert.Equal(t, "", proj.Member("b").Session)
		})
	})

	t.Run("when a session is moved to no group", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored, Moves: []group.Move{{Session: "b", Group: ""}}}, members...)

		t.Run("it should be loose, whatever it shares", func(t *testing.T) {
			_, ok := result.GroupOf("b")
			assert.False(t, ok)
		})
	})

	t.Run("when a likely member is confirmed", func(t *testing.T) {
		likely := []group.Member{
			member("a", "/w/lib", 0, "look around"),
			member("b", "/w/api", 40*time.Second, "open the gateway"),
		}
		result := detect(group.Memory{Confirmed: []string{"b"}}, likely...)
		g, _ := result.GroupOf("b")

		t.Run("it should be sure", func(t *testing.T) {
			assert.Equal(t, group.Sure, g.Member("b").Tier)
		})
	})

	t.Run("when moves and confirmations are saved and loaded", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		saved := group.Memory{
			Moves:       []group.Move{{Session: "b", Group: "g-log"}, {Session: "e", Group: ""}},
			Confirmed:   []string{"c"},
			LikelySince: map[string]time.Time{"d": t0.UTC()},
		}
		require.NoError(t, group.Save(path, saved))
		loaded, err := group.Load(path)
		require.NoError(t, err)

		t.Run("it should round-trip them", func(t *testing.T) {
			assert.Equal(t, saved, loaded)
		})
	})

	t.Run("when a move is remembered", func(t *testing.T) {
		memory := group.Memory{}.WithMove("b", "g-log").WithMove("b", "")

		t.Run("it should keep only the latest move for the session", func(t *testing.T) {
			assert.Equal(t, []group.Move{{Session: "b", Group: ""}}, memory.Moves)
		})
	})
}
