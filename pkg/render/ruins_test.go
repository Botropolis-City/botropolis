package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Aria, 2026-10-18: a session whose transcript Claude Code has deleted is
// drawn as the ruins of its building. Its heft cannot be read, so the
// ruins carry none of what heft and context draw; the rung still dresses
// its lot, as item 86 made it.
func TestRuins(t *testing.T) {
	gone := func() *city.Building {
		b := kindBuilding(city.OfficeDistrict, 4)
		b.Session.ConversationGone = true
		b.Session.State = state.Working
		b.Session.Compactions, b.Session.Spire, b.Session.ContextPercent = 5, 3, 90
		return b
	}
	g := kitGame(t)
	cam := city.NewCamera()

	t.Run("when a session's transcript is gone", func(t *testing.T) {
		t.Run("it should draw no courses", func(t *testing.T) {
			assert.Zero(t, g.courseLift(cam, gone()))
		})

		t.Run("it should raise no spire", func(t *testing.T) {
			assert.Zero(t, spireSegments(gone()))
		})

		t.Run("it should set no skip at the kerb", func(t *testing.T) {
			assert.Zero(t, shownSkip(gone(), city.Job{}, false))
		})

		t.Run("and its lot is overgrown", func(t *testing.T) {
			ruins := gone()
			ruins.Rung, ruins.Blinds, ruins.BoardedUp, ruins.Overgrown = state.RungOvergrown, true, true, true
			intact := kindBuilding(city.OfficeDistrict, 4)
			intact.Rung, intact.Blinds, intact.BoardedUp, intact.Overgrown = state.RungOvergrown, true, true, true

			t.Run("it should dress the lot as it would an intact building", func(t *testing.T) {
				assert.Equal(t, dressLot(intact, -1), dressLot(ruins, -1))
			})
		})
	})
}
