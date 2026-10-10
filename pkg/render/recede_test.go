package render

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

// The isometric map recedes by pushing every sprite through a colour
// matrix; the top-down map has no sprites to push, only flat fills, so it
// recedes by draining the fill colour instead. Two mechanisms, one
// meaning — and the only way that stays true is to check they agree.
func TestRecedeMatchesTheMatrix(t *testing.T) {
	cases := map[string]color.NRGBA{
		"the ground's green": {0x82, 0x9c, 0x6e, 0xff},
		"a district floor":   {0xc2, 0xc0, 0xb8, 0xff},
		"the plaza":          {0xb3, 0xb3, 0xb6, 0xff},
		"a needs-you amber":  {0xe8, 0xa0, 0x3c, 0xff},
		"the water":          {0x3f, 0x6f, 0xa8, 0xff},
		"black, with no hue": {0x00, 0x00, 0x00, 0xff},
		"white, all of it":   {0xff, 0xff, 0xff, 0xff},
	}
	// Opaque colours only: ColorM.Apply hands back a premultiplied
	// colour, so a translucent case would compare the premultiplication
	// rather than the transform. Alpha is checked on its own below.
	m := recedeMatrix()
	for name, in := range cases {
		t.Run("when "+name+" steps back for a view", func(t *testing.T) {
			t.Run("it should land where the sprite path would put it", func(t *testing.T) {
				r, g, b, a := m.Apply(in).RGBA()
				want := color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
				got := ui.Receded(in)
				require.InDelta(t, float64(want.R), float64(got.R), 1, "red")
				require.InDelta(t, float64(want.G), float64(got.G), 1, "green")
				require.InDelta(t, float64(want.B), float64(got.B), 1, "blue")
			})
		})
	}

	t.Run("when a colour is drained", func(t *testing.T) {
		t.Run("it should keep its alpha, because receding is not fading", func(t *testing.T) {
			assert.Equal(t, uint8(0x80), ui.Receded(color.NRGBA{0x40, 0x80, 0xc0, 0x80}).A)
		})
	})
}
