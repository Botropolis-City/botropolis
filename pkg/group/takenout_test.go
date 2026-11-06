package group_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

// Item 348: the result names, by session, the groups it was taken out
// of by hand, so the city moves its building out of that neighbourhood
// once, at Aria's action, and not again when a guess changes.
func TestTakenOut(t *testing.T) {
	a := member("a", "/w/lib", 0, "start PROJ-123 in the library")
	b := member("b", "/w/api", time.Hour, "bump the client for PROJ-123")
	c := member("c", "/w/web", 2*time.Hour, "PROJ-123 page")
	stored := []group.Stored{{ID: "g1", Members: []string{"a", "b", "c"}}}

	t.Run("when a session was taken out of its stored group", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored}.WithTakenOut("c", "g1"), a, b, c)

		t.Run("it should name the group the session left", func(t *testing.T) {
			assert.Equal(t, map[string][]string{"c": {"g1"}}, result.TakenOut)
		})
	})

	t.Run("when no session was taken out", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored}, a, b, c)

		t.Run("it should name none", func(t *testing.T) {
			assert.Empty(t, result.TakenOut)
		})
	})

	t.Run("when a taken-out session is not among the members", func(t *testing.T) {
		result := detect(group.Memory{Groups: stored}.WithTakenOut("c", "g1"), a, b)

		t.Run("it should name none", func(t *testing.T) {
			assert.Empty(t, result.TakenOut)
		})
	})
}
