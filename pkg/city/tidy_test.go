package city_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 333: a group counts only the members the map draws, and a
// session's card corrects its membership both ways.
func TestGroupsOnTheMap(t *testing.T) {
	member := func(id, dir, gid string) state.Session {
		s := session(id, dir, state.Working)
		s.Group, s.GroupJoined, s.Title = gid, "mentions PROJ-9", "Title "+id
		return s
	}
	scripted := func(s state.Session) state.Session {
		s.Entrypoint = "sdk-cli"
		return s
	}
	gone := func(s state.Session) state.Session {
		s.ConversationGone = true
		return s
	}
	groupOf := func(id string, members ...string) state.Group {
		g := state.Group{ID: id, Name: "group " + id, Members: members}
		for _, m := range members {
			g.MemberDirs = append(g.MemberDirs, "/w/"+m)
			g.MemberTitles = append(g.MemberTitles, "Title "+m)
		}
		return g
	}
	scene := func(sessions []state.Session, groups ...state.Group) *city.Scene {
		s := city.NewScene(city.NewLayout())
		s.SetSnapshot(state.Snapshot{At: now, Sessions: sessions, Groups: groups})
		return s
	}
	shown := func(s *city.Scene, id string) (state.Group, bool) {
		for _, g := range s.Groups() {
			if g.ID == id {
				return g, true
			}
		}
		return state.Group{}, false
	}
	building := func(s *city.Scene, id string) *city.Building {
		for _, b := range s.City().Buildings() {
			if b.Session.ID == id {
				return b
			}
		}
		return nil
	}
	selected := func(t *testing.T, s *city.Scene, id string) {
		t.Helper()
		b := building(s, id)
		require.NotNil(t, b)
		require.True(t, s.Select(b.Session.ID))
	}

	t.Run("when a group has members the map leaves off", func(t *testing.T) {
		s := scene([]state.Session{
			member("a", "/w/a", "g1"), member("b", "/w/b", "g1"),
			scripted(member("c", "/w/c", "g1")), gone(member("d", "/w/d", "g1")),
		}, groupOf("g1", "a", "b", "c", "d", "e"))
		g, ok := shown(s, "g1")
		require.True(t, ok)

		t.Run("it should count and name only the members on the map", func(t *testing.T) {
			assert.Equal(t, [2][]string{{"a", "b"}, {"Title a", "Title b"}}, [2][]string{g.Members, g.MemberTitles})
		})

		t.Run("it should keep each member's directory beside it", func(t *testing.T) {
			assert.Equal(t, []string{"/w/a", "/w/b"}, g.MemberDirs)
		})
	})

	t.Run("when none of a group's members are on the map", func(t *testing.T) {
		s := scene([]state.Session{
			scripted(member("c", "/w/c", "g1")), gone(member("d", "/w/d", "g1")), member("x", "/w/x", ""),
		}, groupOf("g1", "c", "d", "e"))
		_, ok := shown(s, "g1")

		t.Run("it should not offer the group", func(t *testing.T) {
			assert.False(t, ok)
		})

		t.Run("it should draw no line for it", func(t *testing.T) {
			assert.Empty(t, s.City().Lines)
		})

		t.Run("it should leave the snapshot it was given as it was", func(t *testing.T) {
			assert.Equal(t, 3, len(s.RealSnapshot().Groups[0].Members))
		})
	})

	// Item 344: a group with one session on the map is shown, not hidden.
	t.Run("when one of a group's members is on the map", func(t *testing.T) {
		s := scene([]state.Session{
			member("a", "/w/a", "g1"), scripted(member("c", "/w/c", "g1")), member("x", "/w/x", ""),
		}, groupOf("g1", "a", "c", "e"))
		g, ok := shown(s, "g1")

		t.Run("it should offer the group, with that member alone", func(t *testing.T) {
			require.True(t, ok)
			assert.Equal(t, []string{"a"}, g.Members)
		})

		t.Run("it should give that member its badge", func(t *testing.T) {
			require.NotNil(t, building(s, "a").Group)
			assert.Equal(t, "g1", building(s, "a").Group.ID)
		})

		t.Run("its card should name the group", func(t *testing.T) {
			assert.Contains(t, building(s, "a").Card(now).Details, "members  Title a")
		})

		t.Run("it should draw no line for it, its one station leading nowhere", func(t *testing.T) {
			assert.Empty(t, s.Lines())
		})

		t.Run("it should leave the snapshot it was given as it was", func(t *testing.T) {
			assert.Equal(t, 3, len(s.RealSnapshot().Groups[0].Members))
		})
	})

	t.Run("when a resting session is alone in its group on the map", func(t *testing.T) {
		alone := member("a", "/w/a", "g1")
		alone.State, alone.Alive, alone.LastActivity = state.Resting, false, now.Add(-30*24*time.Hour)
		s := scene([]state.Session{alone, member("x", "/w/x", "")}, groupOf("g1", "a", "c"))
		var sessions []state.Session
		for _, b := range s.City().Buildings() {
			sessions = append(sessions, b.Session)
		}

		t.Run("the quiet sweep should leave it", func(t *testing.T) {
			assert.NotContains(t, state.Quiet(sessions, now, 7*24*time.Hour), "a")
		})
	})

	t.Run("when the selection is in a group", func(t *testing.T) {
		s := scene([]state.Session{member("a", "/w/a", "g1"), member("b", "/w/b", "g1")}, groupOf("g1", "a", "b"))
		selected(t, s, "a")

		t.Run("it should offer to take it out of the group", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionTakeOut)
		})

		t.Run("it should offer to rename the group", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionRenameGroup)
		})

		t.Run("its details should name the group's members", func(t *testing.T) {
			assert.Contains(t, building(s, "a").Card(now).Details, "members  Title a, Title b")
		})

		t.Run("and take out is chosen", func(t *testing.T) {
			action, _ := s.Act(city.ActionTakeOut)

			t.Run("it should ask the actor to take that session out of that group", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionTakeOut, SessionID: "a", Group: "g1"}, action)
			})
		})

		t.Run("and rename is chosen", func(t *testing.T) {
			action, _ := s.Act(city.ActionRenameGroup)

			t.Run("it should ask for the group's new name", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionRenameGroup, Group: "g1", GroupName: "group g1"}, action)
			})
		})
	})

	t.Run("when the selection is in a group with a member off the map", func(t *testing.T) {
		s := scene([]state.Session{member("a", "/w/a", "g1"), member("b", "/w/b", "g1"), scripted(member("c", "/w/c", "g1"))},
			groupOf("g1", "a", "b", "c"))
		selected(t, s, "a")

		t.Run("it should offer to close the group", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionCloseGroup)
		})

		t.Run("it should say plainly what closing does", func(t *testing.T) {
			assert.Equal(t, "Packs every session in this group into the depot and takes the group off the board. Unpack any of them to bring it back.",
				city.ActionCloseGroup.Describe())
		})

		t.Run("and close is chosen", func(t *testing.T) {
			action, _ := s.Act(city.ActionCloseGroup)

			t.Run("it should close the group, packing every member, on the map or not", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionCloseGroup, Group: "g1", Sessions: []string{"a", "b", "c"}}, action)
			})
		})

		t.Run("and not every pull request merged, close should not come first", func(t *testing.T) {
			assert.NotEqual(t, city.ActionCloseGroup, s.Actions()[0])
		})
	})

	t.Run("when the selection is in a group whose every pull request merged", func(t *testing.T) {
		merged := func(s state.Session, n int) state.Session {
			s.PRs = []claude.PR{{Number: n, Repository: "o/r", State: claude.PRMerged}}
			return s
		}
		s := scene([]state.Session{merged(member("a", "/w/a", "g1"), 1), merged(member("b", "/w/b", "g1"), 2), member("x", "/w/x", "")},
			groupOf("g1", "a", "b"))
		selected(t, s, "a")

		t.Run("close should come first on its card", func(t *testing.T) {
			assert.Equal(t, city.ActionCloseGroup, s.Actions()[0])
		})
	})

	t.Run("when the selection is loose with a suggested group", func(t *testing.T) {
		loose := member("x", "/w/x", "")
		loose.Suggested, loose.SuggestedWhy = "g1", "started with Title a"
		s := scene([]state.Session{member("a", "/w/a", "g1"), member("b", "/w/b", "g1"), loose}, groupOf("g1", "a", "b"))
		selected(t, s, "x")
		actions := s.Actions()

		t.Run("its card should say which group and why", func(t *testing.T) {
			assert.Equal(t, "perhaps group g1, started with Title a", building(s, "x").Card(now).Group)
		})

		t.Run("it should offer to join it, with move to group beside it", func(t *testing.T) {
			at := slices.Index(actions, city.ActionJoinSuggested)
			assert.True(t, at >= 0 && at+1 < len(actions) && actions[at+1] == city.ActionMoveToGroup, "%v", actions)
		})

		t.Run("and join is chosen", func(t *testing.T) {
			action, _ := s.Act(city.ActionJoinSuggested)

			t.Run("it should move the session into the suggested group", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionMove, SessionID: "x", Group: "g1"}, action)
			})
		})
	})

	t.Run("when a loose session's suggested group is not on the map", func(t *testing.T) {
		loose := member("x", "/w/x", "")
		loose.Suggested, loose.SuggestedWhy = "g9", "started with someone"
		s := scene([]state.Session{loose})
		selected(t, s, "x")

		t.Run("it should offer nothing to join", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionJoinSuggested)
		})

		t.Run("its card should not suggest it", func(t *testing.T) {
			assert.False(t, strings.HasPrefix(building(s, "x").Card(now).Group, "perhaps"))
		})
	})
}
