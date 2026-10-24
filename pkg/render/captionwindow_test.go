package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 243: a caption names only what is on the window. A straight
// drawing draws what is off the frame too, and a caption it laid out
// for a flag off the frame was clamped onto the window's edge, naming
// a flag nobody could see.
func TestCaptionsNameOnlyWhatIsOnTheWindow(t *testing.T) {
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	const width, height = 800.0, 600.0
	flag := func(x, y float64) spriteHit {
		return spriteHit{rect: city.RectAt(x, y, 40, 40), hit: city.Hit{Flag: &city.FlagHit{}}}
	}
	captions := func(t *testing.T, held bool, hits ...spriteHit) []ui.Caption {
		t.Helper()
		th := ui.NewTheme(1)
		f, err := newFaces(th)
		require.NoError(t, err)
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(width, height)
		g := &Game{scene: s, theme: th, faces: f, frameHits: hits}
		return g.explainCaptions(held, true, width, height)
	}

	t.Run("when a kind's first sighting is up", func(t *testing.T) {
		t.Run("and the only one of its kind drawn this frame lies wholly off the window", func(t *testing.T) {
			t.Run("it should caption nothing", func(t *testing.T) {
				assert.Empty(t, captions(t, false, flag(900, 300)))
			})
		})

		t.Run("and one lies off the window and one on it", func(t *testing.T) {
			got := captions(t, false, flag(900, 300), flag(300, 300))
			require.Len(t, got, 1)

			t.Run("it should caption the one on the window", func(t *testing.T) {
				assert.InDelta(t, 320, got[0].Plate.Rect.Center().X, 1)
			})
		})

		t.Run("and the only one crosses the window's edge", func(t *testing.T) {
			t.Run("it should caption it", func(t *testing.T) {
				assert.Len(t, captions(t, false, flag(780, 300)), 1)
			})
		})
	})

	t.Run("when explain mode is held, and a kind is drawn only off the window", func(t *testing.T) {
		t.Run("it should not caption that kind", func(t *testing.T) {
			assert.Empty(t, captions(t, true, flag(900, 300)))
		})
	})
}
