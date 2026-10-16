package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 90: a station waiting on another is drawn unlit.
func TestStationTone(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when a station is unlit", func(t *testing.T) {
		t.Run("it should be drawn in the line's own ink, not its move's tone", func(t *testing.T) {
			assert.Equal(t, colorLineCasing, stationTone(th, city.Station{Move: state.MoveTheirs, Unlit: true}))
		})
	})

	t.Run("when a station is lit", func(t *testing.T) {
		t.Run("it should be drawn in its move's tone", func(t *testing.T) {
			assert.Equal(t, th.Color(ui.MoveTone(state.MoveTheirs)), stationTone(th, city.Station{Move: state.MoveTheirs}))
		})
	})
}
