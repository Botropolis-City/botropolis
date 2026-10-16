package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 90: a line's site office is a square at its head.
func TestSiteOfficeShape(t *testing.T) {
	at := city.Point{X: 100, Y: 80}
	box := officeBox(at, 6)

	t.Run("when a line has a site office", func(t *testing.T) {
		t.Run("it should be drawn as a square, not a station's circle", func(t *testing.T) {
			assert.Equal(t, box.Width(), box.Height())
		})

		t.Run("it should stand where the office is", func(t *testing.T) {
			assert.Equal(t, at, box.Center())
		})

		t.Run("it should be larger than a station", func(t *testing.T) {
			assert.Greater(t, box.Width(), 2*6.0)
		})
	})
}

// Item 90: a station waiting on another is drawn unlit.
func TestStationTone(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when a station is unlit", func(t *testing.T) {
		t.Run("it should be drawn in the line's own ink, not its move's tone", func(t *testing.T) {
			assert.Equal(t, colorLineCasing, stationTone(th, city.Station{Move: state.MoveTheirs, Unlit: true}))
		})
	})

	t.Run("when a station is a pull request opened outside the group", func(t *testing.T) {
		t.Run("it should be drawn grey, not in its move's tone", func(t *testing.T) {
			assert.Equal(t, colorTeammate, stationTone(th, city.Station{Move: state.MoveTheirs, Teammate: &city.Teammate{}}))
		})
	})

	t.Run("when a station is lit", func(t *testing.T) {
		t.Run("it should be drawn in its move's tone", func(t *testing.T) {
			assert.Equal(t, th.Color(ui.MoveTone(state.MoveTheirs)), stationTone(th, city.Station{Move: state.MoveTheirs}))
		})
	})
}
