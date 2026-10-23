package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 161: the strip, the stream column and the timeline
// drew every glyph of theirs every frame, though always on screen and
// most frames unchanged. Each is recorded, baked once it holds and
// blitted while it does, as the panels are.
func TestChromeIsBakedWhileItHolds(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
	home := helpers.ParkedWeek(t, 40, now, "Chrome holds")
	settled := func(t *testing.T) *Drive {
		t.Helper()
		d, err := NewDrive(DriveOptions{Home: home, Now: now})
		require.NoError(t, err)
		d.game.timeline = true
		require.NoError(t, d.Run(60))
		return d
	}
	// drawn runs frames, doing each before the first, and counts the
	// frames a region's words were drawn on, straight or into its bake.
	drawn := func(t *testing.T, d *Drive, key bakeKey, frames int, each func()) int {
		t.Helper()
		if each != nil {
			each()
		}
		n := 0
		for range frames {
			require.NoError(t, d.Tick())
			if p := d.game.panels.held[key]; p != nil && p.drawnAt == d.game.panels.frame {
				n++
			}
		}
		return n
	}
	regions := []struct {
		name string
		key  bakeKey
	}{
		{"the strip", bakeStrip},
		{"the stream column", bakeStream},
		{"the timeline", bakeTimeline},
	}

	for _, r := range regions {
		t.Run("when "+r.name+" has settled and 10 frames pass with nothing changing", func(t *testing.T) {
			d := settled(t)
			require.NotNil(t, d.game.panels.held[r.key], "%s was not drawn through its bake", r.name)

			t.Run("it should draw its words on none of them", func(t *testing.T) {
				assert.Zero(t, drawn(t, d, r.key, 10, nil))
			})
		})
	}

	for _, c := range []struct {
		name, region string
		key          bakeKey
		do           func(d *Drive)
	}{
		// The stream's times are clock times, not relative ones, so what
		// changes its words is an event arriving.
		{"an event arrives", "the stream column", bakeStream, func(d *Drive) {
			s := d.game.scene.Snapshot().Sessions[0]
			d.game.AddEvents([]events.Event{{At: d.game.now(), Kind: events.NeedsYou, SessionID: s.ID, Title: "Chrome holds, again"}})
		}},
		// Only a live session's rates are on the strip.
		{"a rate ticks over", "the strip", bakeStrip, func(d *Drive) {
			snap := d.game.scene.Snapshot()
			snap.Sessions = append([]state.Session(nil), snap.Sessions...)
			snap.Sessions[0].State, snap.Sessions[0].Alive = state.Working, true
			snap.Sessions[0].FreshTokensPerHour += 2_000_000
			d.game.Offer(snap)
		}},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			d := settled(t)
			n := drawn(t, d, c.key, 4, func() { c.do(d) })

			t.Run("it should draw "+c.region+" again", func(t *testing.T) {
				assert.Positive(t, n)
			})
		})
	}

	t.Run("when the timeline is closed", func(t *testing.T) {
		d := settled(t)
		d.game.timeline = false
		require.NoError(t, d.Run(2))

		t.Run("it should let its bake go", func(t *testing.T) {
			assert.Nil(t, d.game.panels.held[bakeTimeline])
		})
	})
}
