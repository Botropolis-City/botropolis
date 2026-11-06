package commands_test

import (
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorMove(t *testing.T) {
	t.Run("when a session is moved to a group", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{Muted: []string{"headers"}}))
		actor := commands.Actor{GroupStore: path}
		require.NoError(t, actor.Do(city.Action{Kind: city.ActionMove, SessionID: "a", Group: "g2"}))
		memory, err := group.Load(path)
		require.NoError(t, err)

		t.Run("it should write the move into the groups' memory", func(t *testing.T) {
			assert.Equal(t, []group.Move{{Session: "a", Group: "g2"}}, memory.Moves)
		})

		t.Run("it should keep what memory already held", func(t *testing.T) {
			assert.Equal(t, []string{"headers"}, memory.Muted)
		})
	})

	t.Run("when a session is moved into a new group by name", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{Muted: []string{"headers"}}))
		actor := commands.Actor{GroupStore: path}
		require.NoError(t, actor.Do(city.Action{Kind: city.ActionMove, SessionID: "a", GroupName: "release"}))
		memory, err := group.Load(path)
		require.NoError(t, err)
		require.Len(t, memory.Groups, 1)
		made := memory.Groups[0]

		t.Run("it should store the group under the name typed, as named by hand", func(t *testing.T) {
			assert.Equal(t, group.Stored{ID: made.ID, Name: "release", Renamed: true}, made)
		})

		t.Run("it should write the move into that group", func(t *testing.T) {
			assert.Equal(t, []group.Move{{Session: "a", Group: made.ID}}, memory.Moves)
		})

		t.Run("it should keep what memory already held", func(t *testing.T) {
			assert.Equal(t, []string{"headers"}, memory.Muted)
		})

		t.Run("and the city groups again", func(t *testing.T) {
			result := group.Detect([]group.Member{{ID: "a", Dir: "/w/lib"}, {ID: "b", Dir: "/w/api"}}, memory)

			t.Run("it should list the new group with the session in it", func(t *testing.T) {
				var found []string
				for _, g := range result.Groups {
					if g.Name == "release" {
						for _, m := range g.Members {
							found = append(found, m.Session)
						}
					}
				}
				assert.Equal(t, []string{"a"}, found)
			})
		})
	})

	// Item 343: a move acts on the team, through the lead.
	t.Run("when a lead is moved", func(t *testing.T) {
		t0 := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
		team := func(leadEnded bool) []group.Member {
			lead := group.Member{ID: "l", Dir: "/w/lead", Started: t0, Team: "crew", Prompts: []string{"PROJ-7 start"}}
			if leadEnded {
				lead.Ended = t0.Add(time.Hour)
			}
			return []group.Member{lead,
				{ID: "w1", Dir: "/w/w1", Started: t0.Add(time.Minute), Team: "crew", Lead: "l", Prompts: []string{"follow"}},
				{ID: "w2", Dir: "/w/w2", Started: t0.Add(2 * time.Minute), Ended: t0.Add(time.Hour), Team: "crew", Lead: "l", Prompts: []string{"follow"}},
				{ID: "x", Dir: "/w/api", Started: t0.Add(3 * time.Hour), Prompts: []string{"PROJ-7 client"}},
				{ID: "c", Dir: "/w/logs", Started: t0.Add(6 * time.Hour), Prompts: []string{"LOG-4 buffer"}},
				{ID: "d", Dir: "/w/config", Started: t0.Add(9 * time.Hour), Prompts: []string{"LOG-4 schema"}},
			}
		}
		stored := []group.Stored{{ID: "g-proj", Members: []string{"l", "w1", "w2", "x"}}, {ID: "g-log", Members: []string{"c", "d"}}}
		groupIDs := func(result group.Result, ids ...string) []string {
			var out []string
			for _, id := range ids {
				g, _ := result.GroupOf(id)
				out = append(out, g.ID)
			}
			return out
		}
		moved := func(t *testing.T, action city.Action) group.Memory {
			t.Helper()
			path := filepath.Join(t.TempDir(), "groups.json")
			require.NoError(t, group.Save(path, group.Memory{Groups: stored}))
			require.NoError(t, commands.Actor{GroupStore: path}.Do(action))
			memory, err := group.Load(path)
			require.NoError(t, err)
			return memory
		}

		t.Run("and the city groups again", func(t *testing.T) {
			memory := moved(t, city.Action{Kind: city.ActionMove, SessionID: "l", Group: "g-log"})

			t.Run("it should record one move, the lead's", func(t *testing.T) {
				assert.Equal(t, []group.Move{{Session: "l", Group: "g-log"}}, memory.Moves)
			})

			t.Run("it should move every teammate with it, the resting one included", func(t *testing.T) {
				assert.Equal(t, []string{"g-log", "g-log", "g-log"}, groupIDs(group.Detect(team(false), memory), "l", "w1", "w2"))
			})
		})

		t.Run("and the lead is resting, as a packed lead is", func(t *testing.T) {
			memory := moved(t, city.Action{Kind: city.ActionMove, SessionID: "l", Group: "g-log"})

			t.Run("it should still move every teammate with it", func(t *testing.T) {
				assert.Equal(t, []string{"g-log", "g-log", "g-log"}, groupIDs(group.Detect(team(true), memory), "l", "w1", "w2"))
			})
		})

		t.Run("and it is taken out of its group instead", func(t *testing.T) {
			memory := moved(t, city.Action{Kind: city.ActionTakeOut, SessionID: "l", Group: "g-proj"})
			result := group.Detect(team(false), memory)

			t.Run("it should leave the group to the rest", func(t *testing.T) {
				g, _ := result.GroupOf("x")
				assert.Equal(t, "", g.Member("l").Session+g.Member("w1").Session+g.Member("w2").Session)
			})

			t.Run("it should keep the team together", func(t *testing.T) {
				ids := groupIDs(result, "l", "w1", "w2")
				assert.True(t, ids[0] != "g-proj" && ids[1] == ids[0] && ids[2] == ids[0], "%v", ids)
			})
		})
	})

	t.Run("when the actor keeps no groups, as for a run pointed at another home", func(t *testing.T) {
		err := commands.Actor{}.Do(city.Action{Kind: city.ActionMove, SessionID: "a", Group: "g2"})

		t.Run("it should refuse and say why", func(t *testing.T) {
			assert.ErrorContains(t, err, "groups are kept only for your own home")
		})
	})
}
