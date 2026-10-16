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

	right := func(s *city.Scene) float64 {
		cam, ext := s.Camera(), s.City().Extent()
		most := math.Inf(-1)
		for _, p := range []city.Point{ext.Min, ext.Max, {X: ext.Min.X, Y: ext.Max.Y}, {X: ext.Max.X, Y: ext.Min.Y}} {
			most = math.Max(most, cam.WorldToScreen(p).X)
		}
		return most
	}
	const column = 300.0

	t.Run("when the stream takes the right edge", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou), session("b", botropolis, state.Working))
		s.SetRightChrome(column)
		s.Fit()

		t.Run("a fit should keep the city left of it", func(t *testing.T) {
			assert.LessOrEqual(t, right(s), s.Size().X-column+1)
		})
	})

	t.Run("when the stream is hidden", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou), session("b", botropolis, state.Working))
		s.SetRightChrome(column)
		s.Fit()
		with := right(s)
		s.SetRightChrome(0)
		s.Fit()

		t.Run("a fit should give the city the full width", func(t *testing.T) {
			assert.Greater(t, right(s), with)
		})
	})
}
