package state

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/group"
)

// Item 205: groupViews reads each group's present members through one
// buffer, so one group's members must never stand in another's.
func TestGroupViewsKeepGroupsApart(t *testing.T) {
	t.Run("when one group has a session that needs you and the next has only resting ones", func(t *testing.T) {
		sessions := []Session{
			{ID: "aaaaaaaa-0000-4205-8000-000000000001", Title: "Asks", State: NeedsYou},
			{ID: "aaaaaaaa-0000-4205-8000-000000000002", Title: "Rests", State: Resting},
		}
		result := group.Result{Groups: []group.Group{
			{ID: "g1", Name: "first", Members: []group.Membership{{Session: sessions[0].ID}}},
			{ID: "g2", Name: "second", Members: []group.Membership{{Session: sessions[1].ID}}},
		}}
		views := groupViews(result, nil, sessions)
		moves := map[string]Move{}
		for _, v := range views {
			moves[v.ID] = v.Move
		}

		t.Run("it should give the second group its own move", func(t *testing.T) {
			assert.Equal(t, MoveNobody, moves["g2"])
		})
	})
}

// Item 333: a loose session's card offers the group it might belong to.
func TestGroupViewsCarrySuggestions(t *testing.T) {
	t.Run("when a loose session has a possible-tier suggestion", func(t *testing.T) {
		sessions := []Session{
			{ID: "aaaaaaaa-0000-4333-8000-000000000001", Title: "Limiter", State: Working},
			{ID: "aaaaaaaa-0000-4333-8000-000000000002", Title: "Client", State: Working},
			{ID: "aaaaaaaa-0000-4333-8000-000000000003", Title: "Loose", State: Working},
		}
		result := group.Result{
			Groups: []group.Group{{ID: "g1", Name: "headers", Members: []group.Membership{
				{Session: sessions[0].ID}, {Session: sessions[1].ID}}}},
			Suggestions: []group.Suggestion{{Session: sessions[2].ID, Group: "g1",
				Reason: group.Reason{Signal: group.SignalTiming, With: sessions[0].ID}}},
		}
		groupViews(result, firstSuggestions(result.Suggestions), sessions)

		t.Run("it should carry the suggested group and why on the session", func(t *testing.T) {
			assert.Equal(t, [2]string{"g1", group.Reason{Signal: group.SignalTiming, With: sessions[0].ID}.Describe(TitleOf(sessions))},
				[2]string{sessions[2].Suggested, sessions[2].SuggestedWhy})
		})
	})

	t.Run("when a loose session has two suggestions", func(t *testing.T) {
		const (
			loose = "aaaaaaaa-0000-4340-8000-000000000001"
			alpha = "aaaaaaaa-0000-4340-8000-00000000000a"
			beta  = "aaaaaaaa-0000-4340-8000-00000000000b"
		)
		suggest := func(g string, signal group.Signal, with, detail string) group.Suggestion {
			return group.Suggestion{Session: loose, Group: g, Reason: group.Reason{Signal: signal, With: with, Detail: detail}}
		}
		for _, c := range []struct {
			differ        string
			first, second group.Suggestion
		}{
			{"group", suggest("g1", group.SignalTiming, beta, "y"), suggest("g2", group.SignalBranch, alpha, "x")},
			{"signal", suggest("g1", group.SignalBranch, beta, "y"), suggest("g1", group.SignalTiming, alpha, "x")},
			{"session it matched", suggest("g1", group.SignalWording, alpha, "y"), suggest("g1", group.SignalWording, beta, "x")},
			{"detail", suggest("g1", group.SignalWording, alpha, "x"), suggest("g1", group.SignalWording, alpha, "y")},
		} {
			t.Run("and they differ first by "+c.differ, func(t *testing.T) {
				for _, order := range [][]group.Suggestion{{c.first, c.second}, {c.second, c.first}} {
					sessions := []Session{
						{ID: loose, Title: "Loose"}, {ID: alpha, Title: "Alpha"}, {ID: beta, Title: "Beta"},
					}
					groupViews(group.Result{Suggestions: order}, firstSuggestions(order), sessions)

					t.Run("it should carry the one first by group, then signal, then the session it matched, then detail, in either order", func(t *testing.T) {
						assert.Equal(t, c.first.Reason.Describe(TitleOf(sessions)), sessions[0].SuggestedWhy)
					})
				}
			})
		}
	})
}

// Item 340: the views are rebuilt on every rescan, so what they cost
// grows with the sessions on the map.
func TestGroupViewsCost(t *testing.T) {
	t.Run("when 200 groups of three members are built", func(t *testing.T) {
		const groups, size = 200, 3
		sessions := make([]Session, 0, groups*size)
		result := group.Result{}
		for g := 0; g < groups; g++ {
			view := group.Group{ID: fmt.Sprintf("g%d", g), Name: "group"}
			for k := 0; k < size; k++ {
				id := fmt.Sprintf("aaaaaaaa-0000-4340-8000-%012d", g*size+k)
				sessions = append(sessions, Session{ID: id, Title: "member " + id[len(id)-4:]})
				view.Members = append(view.Members, group.Membership{Session: id,
					Reason: group.Reason{Signal: group.SignalTiming, With: sessions[g*size].ID}})
			}
			result.Groups = append(result.Groups, view)
		}
		allocs := testing.AllocsPerRun(5, func() { groupViews(result, nil, sessions) })

		t.Run("it should describe each member's reason once, at most three allocations a member", func(t *testing.T) {
			assert.LessOrEqual(t, allocs, float64(3*groups*size))
		})
	})
}
