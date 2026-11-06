package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 348, slice 2: the ground layer is drawn again when a
// neighbourhood changes, since it is painted there.
func TestNeighbourhoodGround(t *testing.T) {
	t.Run("when a neighbourhood's member is guessed out of its group", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		sessions := inOrder(in("a", "g1"), in("b", "g1"), loose("c"))
		s.SetSnapshot(withGroups(sessions))
		before := s.GroundGeneration()
		sessions[1].Group = ""
		s.SetSnapshot(withGroups(sessions))

		t.Run("it should move the ground's generation", func(t *testing.T) {
			assert.NotEqual(t, before, s.GroundGeneration())
		})
	})
}
