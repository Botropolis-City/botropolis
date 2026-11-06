package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 343: a move or a take-out of any session in a team acts on the
// team, and the card says so.
func TestTeamActions(t *testing.T) {
	member := func(id, dir string, st state.State) state.Session {
		s := session(id, dir, st)
		s.Group, s.GroupJoined, s.Title = "g1", "mentions PROJ-9", "Title "+id
		return s
	}
	lead := member("l", "/w/lead", state.Working)
	lead.Team = "crew"
	mate := member("w", "/w/mate", state.Resting)
	mate.Team, mate.Agent, mate.Lead = "crew", "worker", "l"
	loner := member("x", "/w/x", state.Working)
	g1 := state.Group{ID: "g1", Name: "group g1", Members: []string{"l", "w", "x"},
		MemberDirs: []string{"/w/lead", "/w/mate", "/w/x"}, MemberTitles: []string{"Title l", "Title w", "Title x"}}
	g2 := state.Group{ID: "g2", Name: "group g2"}
	scene := func(t *testing.T, selected string) *city.Scene {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{lead, mate, loner}, Groups: []state.Group{g1, g2}})
		require.True(t, s.Select(selected))
		return s
	}
	// hintFor is the line the action menu shows for a verb on the
	// selection, which is where the card's verbs are since item 350.
	hintFor := func(t *testing.T, s *city.Scene, kind city.ActionKind) string {
		t.Helper()
		for _, k := range s.Verbs().All() {
			if k == kind {
				return s.DescribeVerb(kind)
			}
		}
		require.Fail(t, "no action "+string(kind))
		return ""
	}

	t.Run("when a resting teammate is selected", func(t *testing.T) {
		s := scene(t, "w")

		t.Run("it should offer to move it, resting as it is, since the move acts on the team", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionMoveToGroup)
		})

		t.Run("it should offer to take it out", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionTakeOut)
		})

		t.Run("its move hint should say it moves the team", func(t *testing.T) {
			assert.Equal(t, "Moves this session and its team into a group, or into a new one you name.", hintFor(t, s, city.ActionMoveToGroup))
		})

		t.Run("its take-out hint should say it takes the team out", func(t *testing.T) {
			assert.Equal(t, "Takes this session and its team out of their group, and keeps them out.", hintFor(t, s, city.ActionTakeOut))
		})

		t.Run("and move to group is chosen", func(t *testing.T) {
			action, _ := s.Act(city.ActionMoveToGroup)

			t.Run("it should ask for the group to move the lead to", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionMoveToGroup, SessionID: "l"}, action)
			})
		})

		t.Run("and take out is chosen", func(t *testing.T) {
			action, note := s.Act(city.ActionTakeOut)

			t.Run("it should ask the actor to take the lead out of the group", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionTakeOut, SessionID: "l", Group: "g1"}, action)
			})

			t.Run("it should say the team went with it", func(t *testing.T) {
				assert.Equal(t, "took Title w and its team out of group g1", note)
			})
		})
	})

	t.Run("when the lead is selected", func(t *testing.T) {
		s := scene(t, "l")

		t.Run("its move hint should say it moves the team", func(t *testing.T) {
			assert.Contains(t, hintFor(t, s, city.ActionMoveToGroup), "and its team")
		})

		t.Run("and take out is chosen", func(t *testing.T) {
			action, _ := s.Act(city.ActionTakeOut)

			t.Run("it should ask the actor to take it out, as itself", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionTakeOut, SessionID: "l", Group: "g1"}, action)
			})
		})
	})

	t.Run("when a session in no team is selected", func(t *testing.T) {
		s := scene(t, "x")

		t.Run("its hints should read as they always have", func(t *testing.T) {
			assert.Equal(t, city.ActionMoveToGroup.Describe(), hintFor(t, s, city.ActionMoveToGroup))
		})
	})

	t.Run("when an action on a session reaches the scene from elsewhere, as a drop or a dive does", func(t *testing.T) {
		s := scene(t, "x")
		cases := []struct {
			scenario string
			action   city.Action
			expected city.Action
		}{
			{"and it moves a teammate", city.Action{Kind: city.ActionMove, SessionID: "w", Group: "g2"}, city.Action{Kind: city.ActionMove, SessionID: "l", Group: "g2"}},
			{"and it asks where to move a teammate", city.Action{Kind: city.ActionMoveToGroup, SessionID: "w"}, city.Action{Kind: city.ActionMoveToGroup, SessionID: "l"}},
			{"and it takes a teammate out", city.Action{Kind: city.ActionTakeOut, SessionID: "w", Group: "g1"}, city.Action{Kind: city.ActionTakeOut, SessionID: "l", Group: "g1"}},
			{"and it moves the lead", city.Action{Kind: city.ActionMove, SessionID: "l", Group: "g2"}, city.Action{Kind: city.ActionMove, SessionID: "l", Group: "g2"}},
			{"and it moves a session in no team", city.Action{Kind: city.ActionMove, SessionID: "x", Group: "g2"}, city.Action{Kind: city.ActionMove, SessionID: "x", Group: "g2"}},
			{"and it packs a teammate", city.Action{Kind: city.ActionPack, SessionID: "w"}, city.Action{Kind: city.ActionPack, SessionID: "w"}},
		}
		for _, c := range cases {
			t.Run(c.scenario, func(t *testing.T) {
				t.Run("it should act on the lead for a move or a take-out, and on the session itself otherwise", func(t *testing.T) {
					assert.Equal(t, c.expected, s.ForTeam(c.action))
				})
			})
		}
	})

	t.Run("when the team's group is closed", func(t *testing.T) {
		s := scene(t, "l")
		action, _ := s.CloseGroup("g1")

		t.Run("it should pack the teammates with the rest", func(t *testing.T) {
			assert.Equal(t, []string{"l", "w", "x"}, action.Sessions)
		})
	})
}
