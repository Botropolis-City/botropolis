package ui_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/ui"
)

func someEvents() []events.Event {
	t0 := time.Date(2026, 9, 18, 14, 5, 0, 0, time.UTC)
	return []events.Event{
		{At: t0.Add(2 * time.Minute), Kind: events.Merged, SessionID: "a", Title: "Fix the CI queue", Detail: "#7 mCedar/cinders"},
		{At: t0.Add(time.Minute), Kind: events.Error, SessionID: "b", Title: "Scaffold", Detail: "2 api errors"},
		{At: t0, Kind: events.NeedsYou, SessionID: "c", Title: "Issue 613", Detail: ""},
	}
}

func TestLayoutTimeline(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when the timeline is laid out", func(t *testing.T) {
		p := ui.LayoutTimeline(th, 900, 700, "Timeline", someEvents(), 1, measure7)

		t.Run("it should centre the panel", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 450, Y: 350}, p.Rect.Center())
		})

		t.Run("it should title it", func(t *testing.T) {
			assert.Equal(t, "Timeline", p.Title.Text)
		})

		t.Run("it should lay one row per event, newest first", func(t *testing.T) {
			require.Len(t, p.Rows, 3)
			assert.Equal(t, "Fix the CI queue", p.Rows[0].Title.Text)
		})

		t.Run("it should stamp each row with the time", func(t *testing.T) {
			assert.Equal(t, "14:07", p.Rows[0].Time.Text)
		})

		t.Run("it should tone each row by its kind", func(t *testing.T) {
			assert.Equal(t, ui.ToneMerged, p.Rows[0].Tone)
			assert.Equal(t, ui.ToneError, p.Rows[1].Tone)
			assert.Equal(t, ui.ToneNeedsYou, p.Rows[2].Tone)
		})

		t.Run("it should say what happened", func(t *testing.T) {
			assert.Equal(t, "merged #7 mCedar/cinders", p.Rows[0].Detail.Text)
			assert.Equal(t, "needs you", p.Rows[2].Detail.Text)
		})

		t.Run("it should flag the cursor's row", func(t *testing.T) {
			assert.True(t, p.Rows[1].Selected)
			assert.False(t, p.Rows[0].Selected)
		})

		t.Run("it should stack rows one under the other", func(t *testing.T) {
			assert.Equal(t, p.Rows[0].Rect.Max.Y, p.Rows[1].Rect.Min.Y)
		})

		t.Run("it should find a row under a point", func(t *testing.T) {
			row, ok := p.Hit(p.Rows[2].Rect.Center())
			require.True(t, ok)
			assert.Equal(t, "c", row.SessionID)
		})
	})

	t.Run("when there are no events", func(t *testing.T) {
		p := ui.LayoutTimeline(th, 900, 700, "Timeline", nil, 0, measure7)

		t.Run("it should say so in one row", func(t *testing.T) {
			require.Len(t, p.Rows, 1)
			assert.Equal(t, "nothing yet", p.Rows[0].Title.Text)
		})
	})

	t.Run("when there are more events than fit", func(t *testing.T) {
		var many []events.Event
		for i := 0; i < 60; i++ {
			many = append(many, events.Event{Kind: events.Started, Title: "s"})
		}
		p := ui.LayoutTimeline(th, 900, 400, "Timeline", many, 0, measure7)

		t.Run("it should drop the rows past the bottom", func(t *testing.T) {
			assert.Less(t, len(p.Rows), 60)
		})
	})
}
