package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Drawing the city in an info view.
//
// The view is subtractive: it shows one dimension and takes the rest
// away. Two things make that readable. The city under it recedes — it
// is drained of colour and darkened, because saturated colour carries
// more weight than muted and the overlay only reads if the base steps
// back. And each object that has a value in this view is painted in the
// view's own colour over the top.
//
// Something with no value in the view is left receding rather than
// painted at one end of the scale. An unknown cost is not a cost of
// nothing, and a session that calls no servers is not calling the first
// one in the list.

// blitBase puts the composed city on the screen, drained of colour
// when a view is up. Saturation is what carries visual weight, so
// taking it out of the base is what lets the overlay read — and it is
// done here, once, on the way to the screen, rather than tinted into
// every layer that went into it.
func (g *Game) blitBase(dst, under *ebiten.Image) {
	if !g.viewing() {
		dst.DrawImage(under, &ebiten.DrawImageOptions{})
		return
	}
	var m colorm.ColorM
	m.ChangeHSV(0, ui.Recede, ui.RecedeValue)
	colorm.DrawImage(dst, under, m, &colorm.DrawImageOptions{})
}

// recedeMatrix drains a sprite the same way the base is drained.
func recedeMatrix() colorm.ColorM {
	var m colorm.ColorM
	m.ChangeHSV(0, ui.Recede, ui.RecedeValue)
	return m
}

// viewTint is how an object should be painted in the current view: the
// colour, and whether there is one at all. Attention has none, and the
// city is drawn as it always was.
func (g *Game) viewTint(b *city.Building) (color.NRGBA, bool) {
	v := g.scene.View()
	if v.Scale() == city.ScaleNone {
		return color.NRGBA{}, false
	}
	t := g.scene.Tint(b)
	if !t.Known {
		return color.NRGBA{}, false
	}
	if v.Scale() == city.ScaleCategory {
		return ui.Category(t.Category), true
	}
	return ui.Sequential.At(t.Value), true
}

// viewScale turns a view colour into the scale a sprite is drawn
// through, so a building keeps its own shading and takes the view's
// hue rather than becoming a flat block of it.
func viewScale(c color.NRGBA) *ebiten.ColorScale {
	tint := &ebiten.ColorScale{}
	tint.Scale(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, 1)
	return tint
}

// viewing reports whether a view other than Attention is up, which is
// when the base recedes.
func (g *Game) viewing() bool { return g.scene.View().Scale() != city.ScaleNone }

// flat is a world colour with the view's recede applied when a view is
// up. It is the top-down projection's half of the subtraction: the
// isometric map pushes sprites through a colour matrix, and there are no
// sprites here to push, only fills.
//
// Applied at the call site rather than inside g.rect, because the
// buildings are the exception — they carry the view's own colour — and a
// primitive that quietly drained everything would make that exception
// invisible.
func (g *Game) flat(c color.NRGBA) color.NRGBA {
	if !g.viewing() {
		return c
	}
	return ui.Receded(c)
}

// flatBody is the colour a building is filled with in the top-down
// projection, once the view and the highlight have had their say. It is
// the flat twin of the tint switch in isoBuilding, and the two have to
// agree about precedence or the same city reads differently in the two
// projections: the highlight fades what is not the answer, a view paints
// what it has something to say about and steps the rest back, and only
// then does the state colour get a look in.
func (g *Game) flatBody(b *city.Building, state color.NRGBA) color.NRGBA {
	switch {
	case g.unlit(b):
		return ui.Receded(state)
	case g.viewing():
		if c, ok := g.viewTint(b); ok {
			return c
		}
		return ui.Receded(state)
	}
	return state
}

// shows reports whether the view up now draws that network. Attention
// draws none of them, which is both the whole point of the view and
// where the frame time goes: the networks are the movers.
func (g *Game) shows(n city.Network) bool { return g.scene.View().Shows(n) }

// unlit reports whether the contextual highlight is up and this building
// is not part of the answer.
//
// It fades rather than recedes, and the difference is the point. A view's
// recede is the city stepping back for as long as the mode is on; the
// highlight is a transient answer to "what is this tied to", so the
// things that are not the answer get out of the way harder and come
// straight back when the pointer moves.
func (g *Game) unlit(b *city.Building) bool {
	return g.lit.Active && !g.lit.HasBuilding(b)
}

// unlitTower is the same question for a tower.
func (g *Game) unlitTower(t *city.Tower) bool {
	return g.lit.Active && !g.lit.HasTower(t)
}

// scenery is the tint for the things a view has nothing to say about.
// It is nil on purpose: a nil tint under a view is what tells the
// sprite path to drain the colour out of it, which a scale cannot do.
func (g *Game) scenery() *ebiten.ColorScale { return nil }

// setView changes the view and says which one it is, because a mode you
// entered is a mode you can forget you are in.
//
// The status is the announcement, not the legend: it names the view and
// the question it answers and then gets out of the way. What the colours
// are worth stays on screen for as long as the view does, a row lower.
func (g *Game) setView(v city.View) {
	g.scene.SetView(v)
	g.SetStatus(v.Name() + " — " + v.Question())
}
