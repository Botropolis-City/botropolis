package group_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

// Item 348: the result names the sessions whose group, or none, is a
// move by hand, so the city knows which buildings Aria changed.
func TestMoved(t *testing.T) {
	lead := group.Member{ID: "a", Dir: "/w/lib", Started: t0, Team: "crew", Prompts: []string{"lead"}}
	teammate := group.Member{ID: "b", Dir: "/w/api", Started: t0.Add(time.Hour), Team: "crew", Lead: "a", Prompts: []string{"follow"}}
	stored := []group.Stored{{ID: "g1", Members: []string{"x", "y"}}}
	cases := []struct {
		scenario string
		memory   group.Memory
		members  []group.Member
		moved    []string
	}{
		{
			scenario: "a session was moved into a group by hand",
			memory:   group.Memory{Groups: stored, Moves: []group.Move{{Session: "a", Group: "g1"}}},
			members:  []group.Member{member("a", "/w/lib", 0, "lead")},
			moved:    []string{"a"},
		},
		{
			scenario: "a session was moved to none",
			memory:   group.Memory{Moves: []group.Move{{Session: "a", Group: ""}}},
			members:  []group.Member{member("a", "/w/lib", 0, "start PROJ-123"), member("b", "/w/api", time.Hour, "bump for PROJ-123")},
			moved:    []string{"a"},
		},
		{
			scenario: "a lead was moved, and it has teammates",
			memory:   group.Memory{Groups: stored, Moves: []group.Move{{Session: "a", Group: "g1"}}},
			members:  []group.Member{lead, teammate},
			moved:    []string{"a", "b"},
		},
		{
			scenario: "a teammate's move was recorded",
			memory:   group.Memory{Groups: stored, Moves: []group.Move{{Session: "b", Group: "g1"}}},
			members:  []group.Member{lead, teammate},
			moved:    []string{"a", "b"},
		},
		{
			scenario: "a session joined on a guess",
			members:  []group.Member{member("a", "/w/lib", 0, "start PROJ-123"), member("b", "/w/api", time.Hour, "bump for PROJ-123")},
			moved:    nil,
		},
		{
			scenario: "a moved session is not among the members",
			memory:   group.Memory{Groups: stored, Moves: []group.Move{{Session: "gone", Group: "g1"}}},
			members:  []group.Member{member("a", "/w/lib", 0, "lead")},
			moved:    nil,
		},
	}
	for _, c := range cases {
		t.Run("when "+c.scenario, func(t *testing.T) {
			result := detect(c.memory, c.members...)

			t.Run("it should name exactly the sessions moved by hand, sorted", func(t *testing.T) {
				assert.Equal(t, c.moved, result.Moved)
			})
		})
	}
}
