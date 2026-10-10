package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// Bug 43. The footer's verbs answer the mouse as well as the keyboard,
// and a click stands in for its key rather than calling the action
// itself: Game.just reads g.clicked alongside the real keyboard, so the
// click runs the keyboard's own path. A second copy of each action
// would be two things that must agree with nothing forcing them to,
// which is the shape this project has paid for six times.
//
// It also keeps Aria's reason intact — "it enables things to be
// discovered while learning controls". The verb goes on showing its
// key, because the key is what the click presses.

// footerSynonyms are the verbs whose chip is not the name of a key.
// Anything not here and not a key name — drag, wheel, click — is not
// clickable, because there is no key for it to press.
var footerSynonyms = map[string]ebiten.Key{
	"?":   ebiten.KeyF1,
	"esc": ebiten.KeyEscape,
}

// clickFooter presses the verb under the cursor, if there is one.
func (g *Game) clickFooter(cursor city.Point) bool {
	g.mu.Lock()
	bar := g.footerBar
	g.mu.Unlock()
	hit, ok := bar.Hit(cursor)
	if !ok {
		return false
	}
	key, ok := press(hit.Key)
	if !ok {
		// A verb with no key behind it — drag, wheel, click. The click
		// still lands on the bar rather than falling through to the
		// map, because the bar is what the pointer is over.
		return true
	}
	g.clicked = key
	return true
}

// press is the key a chip stands for, if any. The footer and the help
// overlay share it so a verb means the same thing wherever it is read.
func press(chip string) (ebiten.Key, bool) {
	if key, ok := footerSynonyms[chip]; ok {
		return key, true
	}
	return keyByName(chip)
}

// clickHelp presses the key on the row under the pointer. The overlay
// closes either way, because it has done its job the moment you have
// picked something off it.
func (g *Game) clickHelp(cursor city.Point) bool {
	if !g.help {
		return false
	}
	g.mu.Lock()
	panel := g.helpPanel
	g.mu.Unlock()
	if hit, ok := panel.Hit(cursor); ok {
		if key, ok := press(hit.Key); ok {
			g.clicked = key
		}
	}
	g.help = false
	return true
}
