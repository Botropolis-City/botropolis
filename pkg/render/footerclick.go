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
	// The close up button beside the quit prompt (item 86).
	if b, ok := g.quitButton(); ok && b.Rect.Contains(cursor) {
		g.openCloseUp()
		return true
	}
	g.mu.Lock()
	bar, waiting := g.footerBar, g.waitingID
	g.mu.Unlock()
	// The needs-you line has no key: a click on it attaches the session
	// it names, which is the one click the bar counts. Tab walks the
	// rest. Item 91.
	if waiting != "" && bar.WaitingRect.Contains(cursor) {
		g.act(city.Action{Kind: city.ActionAttach, SessionID: waiting})
		return true
	}
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
