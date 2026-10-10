package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// The rover stands at a door, the drone circles a roof, the flag stands
// on one and the smoke rises off it — all four are inside or above a
// building's own footprint, so the pointer resolves to the building
// before any sprite is considered. Without this the five movers are
// hoverable in principle and unreachable in practice.
func TestPickHit(t *testing.T) {
	b := &city.Building{}
	at := city.Point{X: 10, Y: 10}
	box := city.RectAt(0, 0, 20, 20)
	mover := spriteHit{rect: box, hit: city.Hit{Building: b, Subagent: b}}
	plain := spriteHit{rect: box, hit: city.Hit{Building: b}}

	t.Run("when the pointer is on a building and a mover is drawn over it", func(t *testing.T) {
		got, ok := pickHit(city.Hit{Building: b}, []spriteHit{plain, mover}, at)

		t.Run("it should take the mover", func(t *testing.T) {
			require.True(t, ok)
			assert.NotNil(t, got.Subagent)
		})
	})

	t.Run("when the pointer is on a building and no mover is drawn there", func(t *testing.T) {
		_, ok := pickHit(city.Hit{Building: b}, []spriteHit{plain}, at)

		t.Run("it should leave the building hover alone", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when the pointer is on open ground over a tall sprite", func(t *testing.T) {
		got, ok := pickHit(city.Hit{}, []spriteHit{plain}, at)

		t.Run("it should still take the sprite, as it always did", func(t *testing.T) {
			require.True(t, ok)
			assert.Equal(t, b, got.Building)
		})
	})

	t.Run("when two movers overlap", func(t *testing.T) {
		first := spriteHit{rect: box, hit: city.Hit{Building: b, Worker: b}}
		got, ok := pickHit(city.Hit{}, []spriteHit{first, mover}, at)

		t.Run("it should take the one drawn last, which is the one on top", func(t *testing.T) {
			require.True(t, ok)
			assert.NotNil(t, got.Subagent)
		})
	})

	t.Run("when the pointer is nowhere near anything", func(t *testing.T) {
		_, ok := pickHit(city.Hit{}, []spriteHit{plain}, city.Point{X: 500, Y: 500})

		t.Run("it should change nothing", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}
