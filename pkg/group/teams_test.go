package group_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 343: a teammate is in its lead's group, always, and a move acts
// on the team.
func TestTeams(t *testing.T) {
	title := func(id string) string { return "Title " + id }
	// crew is a lead, a, and its teammates b and e, e resting; a and the
	// outsiders x and y share PROJ-7, and the outsiders c and d LOG-4.
	lead := func(prompts ...string) group.Member {
		m := member("a", "/w/lib", 0, prompts...)
		m.Team = "crew"
		return m
	}
	teammate := func(id string, after time.Duration) group.Member {
		m := member(id, "/w/"+id, after, "follow")
		m.Team, m.Lead = "crew", "a"
		return m
	}
	resting := func(m group.Member) group.Member {
		m.Ended = m.Started.Add(time.Hour)
		return m
	}
	outsiders := []group.Member{
		member("x", "/w/api", 2*time.Hour, "PROJ-7 client"),
		member("y", "/w/web", 4*time.Hour, "PROJ-7 page"),
		member("c", "/w/logs", 6*time.Hour, "LOG-4 buffer"),
		member("d", "/w/config", 9*time.Hour, "LOG-4 schema"),
	}
	crew := func() []group.Member {
		return append([]group.Member{lead("PROJ-7 start"), teammate("b", 3*time.Hour), resting(teammate("e", 5*time.Hour))}, outsiders...)
	}
	stored := []group.Stored{{ID: "g-proj", Members: []string{"a", "x", "y", "b", "e"}}, {ID: "g-log", Members: []string{"c", "d"}}}
	groupIDs := func(result group.Result, ids ...string) []string {
		var out []string
		for _, id := range ids {
			g, _ := result.GroupOf(id)
			out = append(out, g.ID)
		}
		return out
	}
	memberIDs := func(result group.Result, id string) []string {
		g, _ := result.GroupOf(id)
		var out []string
		for _, m := range g.Members {
			out = append(out, m.Session)
		}
		return out
	}

	t.Run("when a team is on its own", func(t *testing.T) {
		result := detect(group.Memory{}, lead("start"), teammate("b", 3*time.Hour), resting(teammate("e", 5*time.Hour)))

		t.Run("it should make one group of the team, as it always has", func(t *testing.T) {
			assert.Equal(t, []string{"a", "b", "e"}, memberIDs(result, "a"))
		})

		t.Run("it should say the lead leads the team", func(t *testing.T) {
			g, _ := result.GroupOf("a")
			assert.Equal(t, "leads the team crew", g.Member("a").Reason.Describe(title))
		})
	})

	t.Run("when a team is larger than a group can hold", func(t *testing.T) {
		ms := []group.Member{lead("PROJ-7 start")}
		for i := 0; i <= group.MaxMembers; i++ {
			ms = append(ms, teammate(fmt.Sprintf("t%02d", i), time.Duration(i+1)*time.Minute))
		}
		result := detect(group.Memory{}, append(ms, outsiders...)...)
		g, _ := result.GroupOf("a")

		t.Run("it should keep every teammate in the lead's group", func(t *testing.T) {
			assert.Len(t, g.Members, group.MaxMembers+2+2)
		})

		t.Run("it should leave none of them out as capped", func(t *testing.T) {
			assert.Empty(t, result.Capped)
		})
	})

	t.Run("when a lead and its teammate are marked not related", func(t *testing.T) {
		result := detect(group.Memory{NotRelated: []group.Pair{{"a", "b"}}}, crew()...)

		t.Run("it should keep them in one group", func(t *testing.T) {
			assert.Equal(t, []string{"a", "x", "b", "y", "e"}, memberIDs(result, "a"))
		})
	})

	t.Run("when the lead is moved to another group", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored, Moves: []group.Move{{Session: "a", Group: "g-log"}}}, crew()...)

		t.Run("it should move every teammate with it, the resting one included", func(t *testing.T) {
			assert.Equal(t, []string{"g-log", "g-log", "g-log"}, groupIDs(result, "a", "b", "e"))
		})

		t.Run("it should leave none behind", func(t *testing.T) {
			assert.Equal(t, []string{"x", "y"}, memberIDs(result, "x"))
		})

		t.Run("it should be sure of the teammate", func(t *testing.T) {
			g, _ := result.GroupOf("b")
			assert.Equal(t, group.Sure, g.Member("b").Tier)
		})

		t.Run("it should say the teammate follows its lead", func(t *testing.T) {
			g, _ := result.GroupOf("b")
			assert.Equal(t, group.Reason{Signal: group.SignalTeam, Detail: "crew", With: "a"}, g.Member("b").Reason)
		})

		t.Run("it should word it by the lead's title", func(t *testing.T) {
			g, _ := result.GroupOf("b")
			assert.Equal(t, "follows its lead Title a", g.Member("b").Reason.Describe(title))
		})

		t.Run("it should list the members in the order started", func(t *testing.T) {
			assert.Equal(t, []string{"a", "b", "e", "c", "d"}, memberIDs(result, "c"))
		})
	})

	t.Run("when the lead is moved to no group", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored, Moves: []group.Move{{Session: "a", Group: ""}}}, crew()...)

		t.Run("it should leave every teammate in none", func(t *testing.T) {
			assert.Equal(t, []string{"", "", ""}, groupIDs(result, "a", "b", "e"))
		})
	})

	t.Run("when a teammate's move is recorded as well as the lead's", func(t *testing.T) {
		cases := []struct {
			scenario string
			moves    []group.Move
			expected string
		}{
			{"the teammate's is the later", []group.Move{{Session: "a", Group: "g-proj"}, {Session: "b", Group: "g-log"}}, "g-log"},
			{"the lead's is the later", []group.Move{{Session: "b", Group: "g-log"}, {Session: "a", Group: "g-proj"}}, "g-proj"},
		}
		for _, c := range cases {
			t.Run("and "+c.scenario, func(t *testing.T) {
				result := detect(group.Memory{Groups: stored, Moves: c.moves}, crew()...)

				t.Run("it should move the whole team where the later says", func(t *testing.T) {
					assert.Equal(t, []string{c.expected, c.expected, c.expected}, groupIDs(result, "a", "b", "e"))
				})
			})
		}
	})

	t.Run("when a teammate is taken out of the team's group", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored}.WithTakenOut("b", "g-proj"), crew()...)

		t.Run("it should take the lead and every teammate out of it", func(t *testing.T) {
			assert.Equal(t, []string{"x", "y"}, memberIDs(result, "x"))
		})

		t.Run("it should leave the team in a group of its own", func(t *testing.T) {
			ids := groupIDs(result, "a", "b", "e")
			assert.True(t, ids[0] != "" && ids[0] != "g-proj" && ids[1] == ids[0] && ids[2] == ids[0], "%v", ids)
		})
	})

	t.Run("when the lead is resting, as a packed lead is", func(t *testing.T) {
		ms := crew()
		ms[0] = resting(ms[0])
		result := detect(group.Memory{Groups: stored}, ms...)

		t.Run("it should keep the teammates in its group", func(t *testing.T) {
			assert.Equal(t, []string{"g-proj", "g-proj", "g-proj"}, groupIDs(result, "a", "b", "e"))
		})
	})

	t.Run("when the lead is not among the members", func(t *testing.T) {
		orphans := append([]group.Member{teammate("b", 3*time.Hour), resting(teammate("e", 5*time.Hour))}, outsiders...)
		cases := []struct {
			scenario string
			memory   group.Memory
			expected string
		}{
			{"and its move is recorded", group.Memory{Groups: stored, Moves: []group.Move{{Session: "a", Group: "g-log"}}}, "g-log"},
			{"and its move names a group detection did not find", group.Memory{
				Groups: []group.Stored{{ID: "g-new", Name: "release", Renamed: true}}, Moves: []group.Move{{Session: "a", Group: "g-new"}}}, "g-new"},
			{"and only a stored group holds it", group.Memory{Groups: stored}, "g-proj"},
			{"and a teammate's move is recorded", group.Memory{Groups: stored, Moves: []group.Move{{Session: "e", Group: "g-log"}}}, "g-log"},
			{"and nothing records it", group.Memory{}, ""},
		}
		for _, c := range cases {
			t.Run(c.scenario, func(t *testing.T) {
				result := detect(c.memory, orphans...)

				t.Run("it should put the teammates where the lead is recorded, together", func(t *testing.T) {
					assert.Equal(t, []string{c.expected, c.expected}, groupIDs(result, "b", "e"))
				})
			})
		}

		t.Run("and its move names a group detection did not find", func(t *testing.T) {
			result := detect(group.Memory{Groups: []group.Stored{{ID: "g-new", Name: "release", Renamed: true}},
				Moves: []group.Move{{Session: "a", Group: "g-new"}}}, orphans...)
			g, _ := result.GroupOf("b")

			t.Run("it should make the group from memory, under its name", func(t *testing.T) {
				assert.Equal(t, "release", g.Name)
			})
		})
	})

	t.Run("when a teammate was started from another group", func(t *testing.T) {
		ms := crew()
		ms[1].StartedFrom = "g-log"
		result := detect(group.Memory{Groups: stored}, ms...)

		t.Run("it should still follow its lead", func(t *testing.T) {
			assert.Equal(t, []string{"g-proj", "g-proj"}, groupIDs(result, "a", "b"))
		})
	})

	t.Run("when the team's group is closed", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored, Closed: []string{"g-proj"}}, crew()...)

		t.Run("it should hold no teammate", func(t *testing.T) {
			assert.Equal(t, []string{"", "", ""}, groupIDs(result, "a", "b", "e"))
		})
	})

	t.Run("when a teammate shares a reference with an outsider", func(t *testing.T) {
		ms := crew()
		ms[0] = lead("start")
		ms[1].Prompts = []string{"LOG-4 follow"}
		result := detect(group.Memory{}, ms...)

		t.Run("it should not pull the teammate into the outsider's group", func(t *testing.T) {
			assert.Equal(t, []string{"c", "d"}, memberIDs(result, "c"))
		})

		t.Run("it should keep the teammate with its lead", func(t *testing.T) {
			assert.Equal(t, []string{"a", "b", "e"}, memberIDs(result, "a"))
		})
	})

	t.Run("when a teammate's pair with an outsider is marked not related", func(t *testing.T) {
		result := detect(group.Memory{NotRelated: []group.Pair{{"b", "x"}}}, crew()...)

		t.Run("it should keep the lead apart from that outsider too", func(t *testing.T) {
			g, _ := result.GroupOf("a")
			require.NotEmpty(t, g.Members)
			assert.Equal(t, "", g.Member("x").Session)
		})
	})
}
