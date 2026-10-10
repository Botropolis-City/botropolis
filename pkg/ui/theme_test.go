package ui_test

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

func TestTheme(t *testing.T) {
	sizes := []struct {
		size ui.Size
		px   float64
	}{
		{ui.Small, 12},
		{ui.Body, 14},
		{ui.Title, 16},
		{ui.Display, 20},
	}

	t.Run("when built at 1x", func(t *testing.T) {
		th := ui.NewTheme(1)

		t.Run("it should keep the grid at 8 px", func(t *testing.T) {
			assert.Equal(t, 8.0, th.Grid())
		})

		t.Run("it should keep the radius at 6 px", func(t *testing.T) {
			assert.Equal(t, 6.0, th.Radius())
		})

		t.Run("it should keep the hairline at 1 px", func(t *testing.T) {
			assert.Equal(t, 1.0, th.Hairline())
		})

		for _, s := range sizes {
			t.Run(fmt.Sprintf("it should size %s text at %v px", s.size, s.px), func(t *testing.T) {
				assert.Equal(t, s.px, th.Pt(s.size))
			})
		}
	})

	t.Run("when built at 2x", func(t *testing.T) {
		th := ui.NewTheme(2)

		t.Run("it should double the grid", func(t *testing.T) {
			assert.Equal(t, 16.0, th.Grid())
		})

		t.Run("it should scale a px measure", func(t *testing.T) {
			assert.Equal(t, 24.0, th.Px(12))
		})

		for _, s := range sizes {
			t.Run(fmt.Sprintf("it should double %s text", s.size), func(t *testing.T) {
				assert.Equal(t, 2*s.px, th.Pt(s.size))
			})
		}
	})

	t.Run("when built with no scale", func(t *testing.T) {
		th := ui.NewTheme(0)

		t.Run("it should fall back to 1x", func(t *testing.T) {
			assert.Equal(t, 1.0, th.Scale)
		})
	})

	t.Run("when asked for a tone", func(t *testing.T) {
		th := ui.NewTheme(1)
		tones := []struct {
			tone ui.Tone
			want string
		}{
			{ui.ToneNeedsYou, "amber"},
			{ui.ToneWorking, "blue"},
			{ui.ToneWaiting, "teal"},
			{ui.ToneUnattended, "violet"},
			{ui.ToneParked, "slate"},
			{ui.ToneError, "red"},
			{ui.ToneMerged, "green"},
		}
		for _, tc := range tones {
			t.Run(fmt.Sprintf("it should colour %s %s", tc.tone, tc.want), func(t *testing.T) {
				assert.Equal(t, tc.want, hue(th.Color(tc.tone)))
			})
		}

		t.Run("it should colour no tone as the dim text", func(t *testing.T) {
			assert.Equal(t, th.Palette.Dim, th.Color(ui.ToneNone))
		})

		t.Run("it should make the accent the needs-you amber", func(t *testing.T) {
			assert.Equal(t, th.Palette.Accent, th.Color(ui.ToneNeedsYou))
		})
	})

	t.Run("when a session state is mapped to a tone", func(t *testing.T) {
		states := []struct {
			state state.State
			tone  ui.Tone
		}{
			{state.NeedsYou, ui.ToneNeedsYou},
			{state.Working, ui.ToneWorking},
			{state.Waiting, ui.ToneWaiting},
			{state.Unattended, ui.ToneUnattended},
			{state.Parked, ui.ToneParked},
			{state.State("gone"), ui.ToneNone},
		}
		for _, tc := range states {
			t.Run(fmt.Sprintf("it should map %s to %s", tc.state, tc.tone), func(t *testing.T) {
				assert.Equal(t, tc.tone, ui.StateTone(tc.state))
			})
		}
	})
}

// hue names the colour family the eye reads, so the tests pin the brief's
// palette without pinning exact bytes.
func hue(c color.NRGBA) string {
	r, g, b := int(c.R), int(c.G), int(c.B)
	max, min := r, r
	for _, v := range []int{g, b} {
		if v > max {
			max = v
		}
		if v < min {
			min = v
		}
	}
	switch {
	case max-min < 40:
		return "slate"
	case r == max && g > b && g > r/2:
		return "amber"
	case r == max:
		return "red"
	case g == max && b > r && b > g*3/4:
		return "teal"
	case g == max:
		return "green"
	case b == max && r > g:
		return "violet"
	default:
		return "blue"
	}
}
