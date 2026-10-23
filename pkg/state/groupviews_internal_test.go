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
