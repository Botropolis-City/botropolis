package state

import (
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
		views := groupViews(result, sessions)
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
		groupViews(result, sessions)

		t.Run("it should carry the suggested group and why on the session", func(t *testing.T) {
			assert.Equal(t, [2]string{"g1", group.Reason{Signal: group.SignalTiming, With: sessions[0].ID}.Describe(TitleOf(sessions))},
				[2]string{sessions[2].Suggested, sessions[2].SuggestedWhy})
		})
	})
}
