package ui_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutStream(t *testing.T) {
	th := ui.NewTheme(1)
	const width, height, top, footer = 1600.0, 1000.0, 32.0, 40.0
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	list := []events.Event{
		{At: at.Add(-3 * time.Minute), Kind: events.Started, SessionID: "a", Title: "Add reset headers"},
		{At: at.Add(-time.Minute), Kind: events.NeedsYou, SessionID: "b", Title: "Allow the headers through"},
		{At: at.Add(-2 * time.Minute), Kind: events.Error, SessionID: "c", Title: "Cap the log buffer"},
	}
	area := ui.StreamArea(th, width, height, top, footer)
	s := ui.LayoutStream(th, list, area, measure7)

	t.Run("it should dock along the right edge", func(t *testing.T) {
		assert.Equal(t, width, area.Max.X)
	})

	t.Run("it should sit under the strip", func(t *testing.T) {
		assert.GreaterOrEqual(t, area.Min.Y, top)
	})

	t.Run("it should stop above the minimap", func(t *testing.T) {
		mini, ok := ui.LayoutMinimap(th, width, height, footer)
		require.True(t, ok)
		assert.LessOrEqual(t, area.Max.Y, mini.Min.Y)
	})

	t.Run("it should put the newest first", func(t *testing.T) {
		require.Len(t, s.Rows, 3)
		assert.Equal(t, []string{"b", "c", "a"}, []string{s.Rows[0].Event.SessionID, s.Rows[1].Event.SessionID, s.Rows[2].Event.SessionID})
	})

	t.Run("it should mute routine entries and not flagged ones", func(t *testing.T) {
		assert.Equal(t, []bool{false, false, true}, []bool{s.Rows[0].Muted, s.Rows[1].Muted, s.Rows[2].Muted})
	})

	t.Run("when there are more than fit", func(t *testing.T) {
		var many []events.Event
		for i := range 200 {
			many = append(many, events.Event{At: at.Add(-time.Duration(i) * time.Minute), Kind: events.Started, SessionID: fmt.Sprint(i)})
		}
		laid := ui.LayoutStream(th, many, area, measure7)

		t.Run("it should keep the newest", func(t *testing.T) {
			require.NotEmpty(t, laid.Rows)
			assert.True(t, laid.Rows[0].Event.SessionID == "0" && len(laid.Rows) < 200 && laid.Rows[len(laid.Rows)-1].Rect.Max.Y <= area.Max.Y)
		})
	})

	t.Run("when nothing has happened yet", func(t *testing.T) {
		empty := ui.LayoutStream(th, nil, area, measure7)

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, ui.StreamEmpty, empty.Empty.Text)
		})
	})

	t.Run("when a click lands on an entry", func(t *testing.T) {
		e, ok := s.Hit(s.Rows[1].Rect.Center())

		t.Run("it should give back that event", func(t *testing.T) {
			assert.True(t, ok && e.SessionID == "c")
		})
	})
}
