package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

func TestBadgeLayout(t *testing.T) {
	measure := func(s string) (float64, float64) { return float64(len(s)) * 6, 12 }
	foot := city.Point{X: 100, Y: 200}

	t.Run("when a building is in a group", func(t *testing.T) {
		b := &city.Building{Group: &state.Group{ID: "g1", Short: "PROJ-123", Move: state.MoveTheirs}}
		badge, ok := badgeLayout(b, foot, measure)

		t.Run("it should lay out a badge", func(t *testing.T) {
			assert.True(t, ok)
		})

		t.Run("it should carry the group's short name", func(t *testing.T) {
			assert.Equal(t, "PROJ-123", badge.Text)
		})

		t.Run("it should mark it in the tone of whose move it is", func(t *testing.T) {
			assert.Equal(t, ui.ToneWaiting, badge.Tone)
		})

		t.Run("it should sit below the building's foot", func(t *testing.T) {
			assert.Greater(t, badge.Panel.Min.Y, foot.Y)
		})

		t.Run("it should be centred on the building", func(t *testing.T) {
			assert.InDelta(t, foot.X, badge.Panel.Center().X, 0.5)
		})
	})

	t.Run("when a building is loose", func(t *testing.T) {
		_, ok := badgeLayout(&city.Building{}, foot, measure)

		t.Run("it should lay out no badge", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}
