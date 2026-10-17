package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// toBuildings zooms in past fit and the badge threshold, so the scene
// draws buildings to click, as anyone clicking one has (item 89).
func toBuildings(s *city.Scene) {
	if s.Badged() {
		s.Camera().Zoom = min(city.MaxZoom, max(s.FitZoom(), city.BadgeZoom(s.Camera().Projection))*1.3)
	}
}

// Item 89: at fit a district draws as its badge, and its buildings do
// not draw at all.
func TestBadges(t *testing.T) {
	at := time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := 0; i < 3; i++ {
		sessions = append(sessions, state.Session{ID: fmt.Sprintf("s%d", i), CWD: fmt.Sprintf("/p/d%d", i), State: state.Working, StartedAt: at})
	}
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return at }, time.UTC)
	s.Resize(1200, 800)
	s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
	s.Fit()
	g := &Game{scene: s}
	require.True(t, s.Badged())

	t.Run("when badges are drawn", func(t *testing.T) {
		t.Run("no building should be drawn", func(t *testing.T) {
			assert.False(t, g.drawsBuildings())
		})

		t.Run("each badge should stand inside its district on screen", func(t *testing.T) {
			cam := s.Camera()
			var outside int
			for _, b := range s.Badges() {
				foot := badgeFoot(cam, b)
				if !screenBounds(cam, b.District.Rect).Contains(foot) {
					outside++
				}
			}
			assert.Zero(t, outside)
		})
	})

	t.Run("when the camera is zoomed in past the threshold", func(t *testing.T) {
		toBuildings(s)

		t.Run("buildings should be drawn", func(t *testing.T) {
			assert.True(t, g.drawsBuildings())
		})
	})
}
