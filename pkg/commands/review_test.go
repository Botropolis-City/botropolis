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

func TestActorReview(t *testing.T) {
	t.Run("when the first-run decision arrives, and memory already holds groups and corrections", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{
			Groups:     []group.Stored{{ID: "g-old", Members: []string{"x", "y"}}},
			NotRelated: []group.Pair{{"x", "z"}},
		}))
		review := group.Review{Rows: []group.ReviewRow{{ID: "g-new", Name: "PROJ-1", Keep: true,
			Members: []group.ReviewMember{{Session: "a", Dir: "/w/lib"}, {Session: "b", Dir: "/w/api"}}}}}
		actor := commands.Actor{GroupStore: path, Now: func() time.Time { return time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC) }}
		require.NoError(t, actor.Do(city.Action{Kind: city.ActionReviewGroups, Review: &review}))
		memory, err := group.Load(path)
		require.NoError(t, err)

		t.Run("it should keep the groups it held alongside the new ones", func(t *testing.T) {
			var ids []string
			for _, s := range memory.Groups {
				ids = append(ids, s.ID)
			}
			assert.ElementsMatch(t, []string{"g-old", "g-new"}, ids)
		})

		t.Run("it should keep the corrections it held", func(t *testing.T) {
			assert.Contains(t, memory.NotRelated, group.Pair{"x", "z"})
		})

		t.Run("it should mark the first run reviewed", func(t *testing.T) {
			assert.NotNil(t, memory.Reviewed)
		})
	})
}
