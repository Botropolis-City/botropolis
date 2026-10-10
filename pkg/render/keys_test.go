package render

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScriptedKey(t *testing.T) {
	t.Run("when the name is a key's own name", func(t *testing.T) {
		key, shift, ok := scriptedKey("equal")
		require.True(t, ok)

		t.Run("it should be that key, unshifted", func(t *testing.T) {
			assert.Equal(t, ebiten.KeyEqual, key)
			assert.False(t, shift)
		})
	})

	t.Run("when the name is a question mark", func(t *testing.T) {
		key, shift, ok := scriptedKey("?")
		require.True(t, ok)

		t.Run("it should be slash with shift held", func(t *testing.T) {
			assert.Equal(t, ebiten.KeySlash, key)
			assert.True(t, shift)
		})
	})

	t.Run("when the name is nothing Ebitengine knows", func(t *testing.T) {
		_, _, ok := scriptedKey("bogus")

		t.Run("it should say so", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}

func TestPanFor(t *testing.T) {
	t.Run("when the up arrow is the only key down", func(t *testing.T) {
		pan := panFor(func(k ebiten.Key) bool { return k == ebiten.KeyArrowUp }, 12)

		t.Run("it should walk the view north by one step", func(t *testing.T) {
			assert.Equal(t, city.Point{Y: 12}, pan)
		})
	})

	t.Run("when opposite arrows are both down", func(t *testing.T) {
		down := map[ebiten.Key]bool{ebiten.KeyArrowLeft: true, ebiten.KeyArrowRight: true}
		pan := panFor(func(k ebiten.Key) bool { return down[k] }, 12)

		t.Run("it should cancel out", func(t *testing.T) {
			assert.Equal(t, city.Point{}, pan)
		})
	})

	t.Run("when no arrow is down", func(t *testing.T) {
		pan := panFor(func(ebiten.Key) bool { return false }, 12)

		t.Run("it should leave the view alone", func(t *testing.T) {
			assert.Equal(t, city.Point{}, pan)
		})
	})

	t.Run("when the script presses an arrow rather than a hand holding it", func(t *testing.T) {
		held := panFor(func(k ebiten.Key) bool { return k == ebiten.KeyArrowUp }, keyPanStep)
		scripted := panFor(func(k ebiten.Key) bool { return k == ebiten.KeyArrowUp }, keyPanStep*scriptPanBeat)

		t.Run("it should be worth a beat of holding, since the script gets one frame", func(t *testing.T) {
			assert.Equal(t, held.Y*scriptPanBeat, scripted.Y)
		})
	})
}
