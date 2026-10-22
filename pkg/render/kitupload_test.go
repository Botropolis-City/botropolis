package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 70: the window puts the atlas up a page an update, inside the
// loop, because Ebitengine holds a copy of anything uploaded before the
// loop starts until its first frame.
func TestKitUpload(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.Local)
	manifests, err := assets.LoadKitManifests()
	require.NoError(t, err)
	total := 0
	for _, a := range manifests {
		total += a.PageCount()
	}
	uploaded := func(k *kits) int {
		n := 0
		for _, a := range k.atlases {
			n += len(a.pages)
		}
		return n
	}
	drive := func(t *testing.T) *Drive {
		t.Helper()
		d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 3, now, "Upload the atlas"), Now: now})
		require.NoError(t, err)
		return d
	}
	window := func(t *testing.T) *Drive {
		t.Helper()
		d := drive(t)
		k, err := loadKitsLazily()
		require.NoError(t, err)
		d.game.kits = k
		return d
	}

	t.Run("when the kits are loaded for a window", func(t *testing.T) {
		t.Run("it should hold no page before the first update", func(t *testing.T) {
			assert.Zero(t, uploaded(window(t).game.kits))
		})

		t.Run("and one update runs", func(t *testing.T) {
			d := window(t)
			before := d.game.ticks
			require.NoError(t, d.Tick())

			t.Run("it should hold exactly one page", func(t *testing.T) {
				assert.Equal(t, 1, uploaded(d.game.kits))
			})

			t.Run("it should not have moved the city's clock", func(t *testing.T) {
				assert.Equal(t, before, d.game.ticks)
			})
		})

		t.Run("and updates run until every page is up", func(t *testing.T) {
			d := window(t)
			for range total {
				require.NoError(t, d.Tick())
			}
			up := uploaded(d.game.kits)
			ticksWhileLoading := d.game.ticks
			require.NoError(t, d.Tick())

			t.Run("it should hold every page of every atlas", func(t *testing.T) {
				assert.Equal(t, total, up)
			})

			t.Run("it should start the clock on the next update", func(t *testing.T) {
				assert.Equal(t, ticksWhileLoading+1, d.game.ticks)
			})
		})
	})

	t.Run("when a page goes up", func(t *testing.T) {
		t.Run("it should be its own texture, not packed into a shared one", func(t *testing.T) {
			assert.True(t, pageOptions().Unmanaged)
		})

		// Item 162: Ebitengine keeps an image's written pixels on the CPU
		// until it is first drawn from, and the z2 pages are not drawn
		// below zoom 1.98, so 96 MiB of them stayed on the heap.
		t.Run("it should be drawn from once, so its pixels are sent and let go", func(t *testing.T) {
			k, err := loadKitsLazily()
			require.NoError(t, err)
			sent := map[*ebiten.Image]int{}
			k.send = func(page *ebiten.Image) { sent[page]++ }
			for range total {
				_, err := k.uploadNext()
				require.NoError(t, err)
			}
			want, got := map[*ebiten.Image]int{}, map[*ebiten.Image]int{}
			for _, a := range k.atlases {
				for _, p := range a.pages {
					want[p] = 1
					got[p] = sent[p]
				}
			}
			assert.Equal(t, want, got)
		})
	})

	t.Run("when the kits are loaded for a drive", func(t *testing.T) {
		t.Run("it should hold every page before the first update", func(t *testing.T) {
			assert.Equal(t, total, uploaded(drive(t).game.kits))
		})
	})
}
