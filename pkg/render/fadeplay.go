package render

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// fadePlay lays recorded draws under the hover fade as the fader does
// (item 108): each fade group's draws go onto the fade layer, and the
// layer goes down at the fade's alpha when the group ends, clipped to
// the part of the frame being played (item 193).
type fadePlay struct {
	real, layer *ebiten.Image
	group       int
	dirty       bool
}

// newFadePlay is a fade over real, which may be a clipped part of the
// frame; the layer is the game's, the frame's size, clipped the same.
func (g *Game) newFadePlay(real *ebiten.Image) *fadePlay {
	if !g.lit.RecedesGround() {
		return nil
	}
	size := real.Bounds().Max
	if g.carFadeLayer == nil || g.carFadeLayer.Bounds().Dx() < size.X || g.carFadeLayer.Bounds().Dy() < size.Y {
		if g.carFadeLayer != nil {
			g.carFadeLayer.Deallocate()
		}
		g.carFadeLayer = sizedImage(nil, size)
	}
	layer := g.carFadeLayer.SubImage(real.Bounds()).(*ebiten.Image)
	layer.Clear()
	return &fadePlay{real: real, layer: layer}
}

// target is where a draw in a fade group goes: the layer, or the frame
// for a draw in none, the layer put down first when a group ends.
func (f *fadePlay) target(dst *ebiten.Image, group int) *ebiten.Image {
	if group != f.group {
		f.done()
		f.group = group
	}
	if group == 0 {
		return dst
	}
	f.dirty = true
	return f.layer
}

// done puts down whatever is on the layer; a nil fadePlay has none.
func (f *fadePlay) done() {
	if f == nil || !f.dirty {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(recededAlpha)
	f.real.DrawImage(f.layer, op)
	f.layer.Clear()
	f.dirty = false
}
