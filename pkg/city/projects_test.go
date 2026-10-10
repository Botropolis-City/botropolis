package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestProjects(t *testing.T) {
	t.Run("when projects have live, needing and parked sessions", func(t *testing.T) {
		layout := city.NewLayout()
		layout.SetStarred(cinders, true)
		c := build(t, layout,
			session("a", botropolis, state.Working),
			session("b", cinders, state.Parked),
			session("c", cinders, state.NeedsYou),
			session("d", cinders, state.Working),
			session("e", "/p/app", state.Unattended))
		rows := c.Projects(layout)
		require.Len(t, rows, 3)

		t.Run("it should list starred projects first, then by name", func(t *testing.T) {
			assert.Equal(t, []string{"cinders", "app", "botropolis"}, []string{rows[0].Name, rows[1].Name, rows[2].Name})
		})

		t.Run("it should count live and needs-you sessions", func(t *testing.T) {
			assert.Equal(t, 2, rows[0].Live)
			assert.Equal(t, 1, rows[0].NeedsYou)
		})

		t.Run("it should list a project's sessions by urgency with parked last", func(t *testing.T) {
			ids := []string{}
			for _, b := range rows[0].Sessions {
				ids = append(ids, b.Session.ID)
			}
			assert.Equal(t, []string{"c", "d", "b"}, ids)
		})

		t.Run("it should mark the starred project", func(t *testing.T) {
			assert.True(t, rows[0].Starred)
			assert.False(t, rows[1].Starred)
		})
	})

	t.Run("when the city is empty", func(t *testing.T) {
		t.Run("it should list nothing", func(t *testing.T) {
			assert.Empty(t, build(t, city.NewLayout()).Projects(city.NewLayout()))
		})
	})
}
