package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 115. A recording has no mouse: under xvfb the system cursor sits
// in the middle of the screen, and a pan_to brings its district right
// there, so the district stayed hovered and the rest of the city receded
// for the whole clip. Only a pointer the script shows can hover.

func TestUnshownPointer(t *testing.T) {
	now := time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC)
	const width, height = 1280.0, 800.0

	newGame := func(t *testing.T) (*Game, city.Point) {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(width, height)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{
			{ID: "a", CWD: "/w/tidepool", State: state.Working, StartedAt: now.Add(-time.Hour), LastActivity: now},
			{ID: "b", CWD: "/w/kiln", State: state.Working, StartedAt: now.Add(-time.Hour), LastActivity: now},
		}})
		var ground city.Point
		found := false
		for _, d := range s.City().Districts {
			for y := d.Rect.Min.Y; y <= d.Rect.Max.Y && !found; y += 2 {
				for x := d.Rect.Min.X; x <= d.Rect.Max.X && !found; x += 2 {
					hit := s.City().At(city.Point{X: x, Y: y})
					if hit.District == d && hit.Building == nil {
						ground, found = s.Camera().WorldToScreen(city.Point{X: x, Y: y}), true
					}
				}
			}
			if found {
				break
			}
		}
		require.True(t, found, "no open ground in any district")
		return &Game{scene: s, record: "/frames", scripted: -1, clicked: -1}, ground
	}
	onCursor := func(t *testing.T, at city.Point) {
		t.Helper()
		restore := cursorPosition
		cursorPosition = func() (int, int) { return int(at.X), int(at.Y) }
		t.Cleanup(func() { cursorPosition = restore })
	}

	t.Run("when recording, and the script has panned to a district without showing the pointer, and the system cursor sits on that district", func(t *testing.T) {
		g, district := newGame(t)
		onCursor(t, district)
		g.play = playFrame{}
		g.notePointer()

		t.Run("it should leave nothing highlighted", func(t *testing.T) {
			assert.False(t, g.scene.Highlight().Active)
		})
	})

	t.Run("when recording, and a point_at cue has shown the pointer on the district", func(t *testing.T) {
		g, district := newGame(t)
		onCursor(t, city.Point{X: -100, Y: -100})
		g.play = playFrame{shown: true, pointer: district}
		g.notePointer()

		t.Run("it should highlight the district", func(t *testing.T) {
			assert.True(t, g.scene.Highlight().Active)
		})
	})
}
