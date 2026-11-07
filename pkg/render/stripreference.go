package render

import (
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// referenceHint is what the strip's hover card says the reference's key
// opens (item 353).
const referenceHint = "the reference: every key"

// stripHoverCard is the card for a hovered state chip on the strip,
// ending with the reference's key as it is bound, so the keys can be
// found from the strip (item 353).
func (g *Game) stripHoverCard(st state.State) city.Card {
	card := g.scene.StateCard(st)
	card.Hint = referenceHint
	card.HintKey = []string{g.findChords().Bound["reference"].Label()}
	return card
}
