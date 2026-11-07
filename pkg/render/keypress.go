package render

import "github.com/hajimehoshi/ebiten/v2"

// keySynonyms are the rows whose key is not the name of a key. Anything
// not here and not a key name — drag, wheel, click — is not clickable,
// because there is no key for it to press.
var keySynonyms = map[string]ebiten.Key{
	"?":   ebiten.KeyF1,
	"esc": ebiten.KeyEscape,
}

// press is the key a row stands for, if any, so a clicked row runs the
// keyboard's own path rather than a second copy of it (bug 43).
func press(chip string) (ebiten.Key, bool) {
	if key, ok := keySynonyms[chip]; ok {
		return key, true
	}
	return keyByName(chip)
}
