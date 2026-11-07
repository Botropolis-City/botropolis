package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
)

// Item 351: a key is held the same whichever way it is held — by hand,
// by hold+ in --keys, or by a recording script's hold — so the hold and
// explain mode can be filmed.
func TestKeyHeld(t *testing.T) {
	for _, tc := range []struct {
		name string
		game *Game
		want bool
	}{
		{"when nothing holds e", &Game{}, false},
		{"when --keys holds e", &Game{scriptedHeld: map[ebiten.Key]bool{ebiten.KeyE: true}}, true},
		{"when a recording script holds e", &Game{play: playFrame{hold: "e"}}, true},
		{"when a recording script holds another key", &Game{play: playFrame{hold: "ArrowLeft"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("it should say whether e is held", func(t *testing.T) {
				assert.Equal(t, tc.want, tc.game.keyHeld(ebiten.KeyE))
			})
		})
	}
}
