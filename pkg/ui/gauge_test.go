package ui_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bug 52. The boats are separated across the river and read along it,
// so the lanes are a pure layout: given the water's width and the
// widest hull that has to ride in it, where does each boat sit.

func TestGaugeLanes(t *testing.T) {
	// The three hulls' beams in lane order: liner, cargo, sail.
	beams := []float64{47.7, 19.8, 17.2}
	const width, margin = 144.0, 8.0

	t.Run("when three boats are laid across a river", func(t *testing.T) {
		lanes := ui.GaugeLanes(width, beams, margin)
		require.Len(t, lanes, 3)

		t.Run("it should space the lanes evenly", func(t *testing.T) {
			assert.InDelta(t, lanes[1]-lanes[0], lanes[2]-lanes[1], 0.0001)
		})

		// Bug 55. The widest hull used to sit exactly half a beam from
		// the bank, so its superstructure lay over the grass — a
		// clearance of zero that fell out of the arithmetic rather than
		// being chosen.
		t.Run("it should leave water between the widest hull and the bank", func(t *testing.T) {
			assert.InDelta(t, margin, width/2+lanes[0]-beams[0]/2, 0.0001)
		})

		t.Run("it should leave the same water at the other bank", func(t *testing.T) {
			assert.InDelta(t, margin, width/2-lanes[2]-beams[2]/2, 0.0001)
		})

		// The narrow boat needs less room than the wide one, and
		// pinning each end by its own hull is what pays for the margin.
		t.Run("it should give the narrow boat's end more room than the wide boat's", func(t *testing.T) {
			assert.Greater(t, lanes[2], -lanes[0])
		})
	})

	t.Run("when a plan reports a single limit", func(t *testing.T) {
		t.Run("it should sail the one boat down the middle rather than off to a side", func(t *testing.T) {
			assert.Equal(t, []float64{0}, ui.GaugeLanes(width, beams[:1], margin))
		})
	})

	t.Run("when the hulls are wider than the water they are given", func(t *testing.T) {
		t.Run("it should still give every boat a lane", func(t *testing.T) {
			assert.Len(t, ui.GaugeLanes(40, beams, margin), 3)
		})
	})
}

func TestGaugeShare(t *testing.T) {
	t.Run("when a reading is an ordinary percentage", func(t *testing.T) {
		t.Run("it should be that share of the run", func(t *testing.T) {
			assert.InDelta(t, 0.4, ui.GaugeShare(40), 0.0001)
		})
	})

	t.Run("when a reading has overrun its limit", func(t *testing.T) {
		t.Run("it should stop at the far end rather than sail off the river", func(t *testing.T) {
			assert.Equal(t, 1.0, ui.GaugeShare(140))
		})
	})

	t.Run("when a reading is somehow negative", func(t *testing.T) {
		t.Run("it should hold at the near end", func(t *testing.T) {
			assert.Equal(t, 0.0, ui.GaugeShare(-5))
		})
	})
}
