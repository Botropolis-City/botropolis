package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 162: z1 is picked below zoom 1.98 and z2 from there, so a window
// holds only the level its camera picks, and lets the other go once it
// has gone unpicked for kitHoldUpdates.
func TestKitResidency(t *testing.T) {
	const out, in = 1.0, 2.0
	type window struct {
		k    *kits
		sent int
	}
	open := func(t *testing.T) *window {
		t.Helper()
		k, err := loadKitsLazily()
		require.NoError(t, err)
		require.Len(t, k.atlases, 2)
		w := &window{k: k}
		k.send = func(*ebiten.Image) { w.sent++ }
		return w
	}
	updates := func(t *testing.T, w *window, zoom float64, n int) {
		t.Helper()
		for range n {
			require.NoError(t, w.k.view(zoom))
		}
	}
	untilUp := func(t *testing.T, w *window, zoom float64) {
		t.Helper()
		for range 64 {
			if w.k.ready() && w.k.pick(zoom) == &w.k.atlases[w.k.index(zoom)] {
				return
			}
			require.NoError(t, w.k.view(zoom))
		}
		t.Fatalf("the level for zoom %g never came up", zoom)
	}
	z1, z2 := 0, 1
	pages := func(w *window, i int) int { return len(w.k.atlases[i].pages) }
	full := func(w *window, i int) int { return w.k.atlases[i].PageCount() }

	t.Run("when a window opens at a zoom that picks z1", func(t *testing.T) {
		w := open(t)
		untilUp(t, w, out)

		t.Run("it should put up every z1 page before the clock starts", func(t *testing.T) {
			assert.Equal(t, full(w, z1), pages(w, z1))
		})

		t.Run("it should put up no z2 page", func(t *testing.T) {
			assert.Zero(t, pages(w, z2))
		})
	})

	t.Run("when the camera zooms to 2", func(t *testing.T) {
		w := open(t)
		untilUp(t, w, out)
		require.NoError(t, w.k.view(in))

		t.Run("it should draw with z1 while z2 goes up", func(t *testing.T) {
			assert.Same(t, &w.k.atlases[z1], w.k.pick(in))
		})

		t.Run("it should put z2 up a page an update", func(t *testing.T) {
			assert.Equal(t, 1, pages(w, z2))
		})

		t.Run("and z2 is all up", func(t *testing.T) {
			w := open(t)
			untilUp(t, w, out)
			untilUp(t, w, in)

			t.Run("it should draw with z2", func(t *testing.T) {
				assert.Same(t, &w.k.atlases[z2], w.k.pick(in))
			})

			t.Run("it should still hold z1", func(t *testing.T) {
				assert.Equal(t, full(w, z1), pages(w, z1))
			})

			t.Run("and the hold passes at that zoom", func(t *testing.T) {
				updates(t, w, in, kitHoldUpdates)

				t.Run("it should have let every z1 page go", func(t *testing.T) {
					assert.Zero(t, pages(w, z1))
				})
			})
		})
	})

	t.Run("when the camera zooms to 2 and back within the hold", func(t *testing.T) {
		w := open(t)
		untilUp(t, w, out)
		before := w.sent
		updates(t, w, in, 2)
		halfUp := pages(w, z2)
		updates(t, w, out, 1)

		t.Run("it should put no z1 page up again", func(t *testing.T) {
			assert.Equal(t, before+halfUp, w.sent)
		})

		t.Run("and the hold passes back at z1", func(t *testing.T) {
			updates(t, w, out, kitHoldUpdates)

			t.Run("it should have let z2's pages go, including those half put up", func(t *testing.T) {
				assert.Zero(t, pages(w, z2))
			})
		})
	})

	t.Run("when a drive loads the kits", func(t *testing.T) {
		k, err := loadKits()
		require.NoError(t, err)
		for range kitHoldUpdates + 1 {
			require.NoError(t, k.view(out))
		}

		t.Run("it should hold every page of every level", func(t *testing.T) {
			held := 0
			for _, a := range k.atlases {
				held += len(a.pages) - a.PageCount()
			}
			assert.Zero(t, held)
		})
	})
}
