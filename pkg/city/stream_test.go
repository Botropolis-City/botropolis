package city_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

func TestStream(t *testing.T) {
	t.Run("for every event kind", func(t *testing.T) {
		flagged := map[events.Kind]bool{events.NeedsYou: true, events.Error: true}
		for _, k := range []events.Kind{events.NeedsYou, events.Error, events.PR, events.Merged, events.Compaction, events.Started, events.Ended} {
			t.Run("when it is "+string(k), func(t *testing.T) {
				t.Run("it should be muted, or flagged if it is needs-you or an error", func(t *testing.T) {
					assert.Equal(t, flagged[k], city.Flagged(k))
				})
			})
		}
	})

	t.Run("when an entry is clicked", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou), session("b", botropolis, state.Working))
		ok := s.FlyToEvent(events.Event{Kind: events.NeedsYou, SessionID: "b"})
		require.True(t, ok)

		t.Run("it should select its session", func(t *testing.T) {
			assert.Equal(t, "b", s.Selected().Session.ID)
		})

		t.Run("it should centre the camera on it", func(t *testing.T) {
			in, size := s.Insets(), s.Size()
			want := city.Point{X: in.Left + (size.X-in.Left-in.Right)/2, Y: in.Top + (size.Y-in.Top-in.Bottom)/2}
			got := s.Camera().WorldToScreen(s.Selected().Rect.Center())
			assert.True(t, math.Abs(got.X-want.X) < 1 && math.Abs(got.Y-want.Y) < 1, "%v, want %v", got, want)
		})

		t.Run("and the session is gone", func(t *testing.T) {
			t.Run("it should say so", func(t *testing.T) {
				assert.False(t, s.FlyToEvent(events.Event{Kind: events.Ended, SessionID: "gone"}))
			})
		})
	})

}

func TestSetCornerChrome(t *testing.T) {
	onScreen := func(s *city.Scene) city.Rect {
		cam, ext := s.Camera(), s.City().Extent()
		r := city.Rect{Min: cam.WorldToScreen(ext.Min), Max: cam.WorldToScreen(ext.Min)}
		for _, p := range []city.Point{ext.Max, {X: ext.Min.X, Y: ext.Max.Y}, {X: ext.Max.X, Y: ext.Min.Y}} {
			at := cam.WorldToScreen(p)
			r = r.Union(city.Rect{Min: at, Max: at})
		}
		return r
	}
	fitted := func(t *testing.T, panel city.Rect) *city.Scene {
		s := scene(t, session("a", cinders, state.NeedsYou), session("b", botropolis, state.Working))
		s.SetCornerChrome(panel)
		s.Fit()
		return s
	}
	const w, h = 200.0, 192.0
	for _, c := range []struct {
		name  string
		panel city.Rect
	}{
		{"top right", city.RectAt(800-w, 32, w, h)},
		{"bottom right", city.RectAt(800-w, 600-40-h, w, h)},
	} {
		t.Run("when a panel is reserved in the "+c.name+" corner", func(t *testing.T) {
			s := fitted(t, c.panel)

			t.Run("it should keep the city's on-screen extent clear of the panel", func(t *testing.T) {
				assert.False(t, onScreen(s).Inset(1).Overlaps(c.panel), "city %v, panel %v", onScreen(s), c.panel)
			})

			t.Run("it should fit the city at least as large as a full-height column of the same width", func(t *testing.T) {
				column := fitted(t, city.RectAt(800-w, 0, w, 600))
				assert.GreaterOrEqual(t, s.Camera().Zoom, column.Camera().Zoom)
			})

			t.Run("and the panel is taken away", func(t *testing.T) {
				s.SetCornerChrome(city.Rect{})
				s.Fit()

				t.Run("it should fit as with no panel", func(t *testing.T) {
					assert.Equal(t, fitted(t, city.Rect{}).Camera().Zoom, s.Camera().Zoom)
				})
			})
		})
	}

	t.Run("when the panel is far wider than it is tall", func(t *testing.T) {
		s := fitted(t, city.RectAt(800-400, 32, 400, 40))

		t.Run("it should fit the city larger than a full-height column of its width would", func(t *testing.T) {
			column := fitted(t, city.RectAt(800-400, 0, 400, 600))
			assert.Greater(t, s.Camera().Zoom, column.Camera().Zoom)
		})
	})
}
