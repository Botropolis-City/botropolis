package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestGroupsMatching(t *testing.T) {
	groups := []state.Group{
		{ID: "g1", Name: "PROJ-123 · rate-limit headers", Short: "PROJ-123", Anchor: "PROJ-123"},
		{ID: "g2", Name: "LOG-88 · buffer cap", Short: "LOG-88", Anchor: "LOG-88"},
		{ID: "g3", Name: "headers for the site", Short: "headers"},
		{ID: "g4", Name: "headers in the docs", Short: "docs"},
		{ID: "g5", Name: "headers in the config", Short: "config"},
	}
	ids := func(gs []state.Group) []string {
		out := []string{}
		for _, g := range gs {
			out = append(out, g.ID)
		}
		return out
	}

	for _, c := range []struct {
		when, query string
		want        []string
	}{
		{"the query is empty", "", []string{}},
		{"the query is a group's anchor", "LOG-88", []string{"g2"}},
		{"the query is a word of a group's name in another case", "RATE-LIMIT", []string{"g1"}},
		{"one of the query's words is in no group", "rate-limit orchard", []string{}},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			t.Run("it should match exactly the groups named by every word", func(t *testing.T) {
				assert.Equal(t, c.want, ids(city.GroupsMatching(groups, c.query)))
			})
		})
	}

	t.Run("when more than three groups match", func(t *testing.T) {
		got := city.GroupsMatching(groups, "headers")

		t.Run("it should keep three", func(t *testing.T) {
			assert.Len(t, got, 3)
		})

		t.Run("it should keep the most recently active first", func(t *testing.T) {
			assert.Equal(t, []string{"g1", "g3", "g4"}, ids(got))
		})
	})
}
