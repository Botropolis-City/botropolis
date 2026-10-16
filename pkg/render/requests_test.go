package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/single"
)

// fakeWindow is a window that keeps what was asked of it.
type fakeWindow struct {
	minimized, canRaise              bool
	restored, raised, attentionAsked int
}

func (w *fakeWindow) Minimized() bool { return w.minimized }
func (w *fakeWindow) Restore()        { w.restored++; w.minimized = false }
func (w *fakeWindow) Raise() bool {
	if w.canRaise {
		w.raised++
	}
	return w.canRaise
}
func (w *fakeWindow) RequestAttention() { w.attentionAsked++ }

func TestRequests(t *testing.T) {
	// asked is a city with a window that hears one request, on its next
	// tick.
	asked := func(t *testing.T, w *fakeWindow, r single.Request) *Game {
		t.Helper()
		requests := make(chan single.Request, 1)
		requests <- r
		g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1, requests: requests, win: w}
		require.NoError(t, g.handleKeys())
		return g
	}

	t.Run("when a request to come forward arrives", func(t *testing.T) {
		t.Run("and the window is minimized", func(t *testing.T) {
			w := &fakeWindow{minimized: true}
			asked(t, w, single.Request{Op: single.Raise})

			t.Run("it should restore it", func(t *testing.T) {
				assert.Equal(t, 1, w.restored)
			})
		})

		t.Run("and the desktop cannot raise windows", func(t *testing.T) {
			w := &fakeWindow{}
			asked(t, w, single.Request{Op: single.Raise})

			t.Run("it should ask for attention", func(t *testing.T) {
				assert.Equal(t, 1, w.attentionAsked)
			})
		})
	})

	t.Run("when a request to find arrives", func(t *testing.T) {
		w := &fakeWindow{canRaise: true}
		g := asked(t, w, single.Request{Op: single.Find, Words: "kiln"})

		t.Run("it should open the finder and come forward", func(t *testing.T) {
			f, open := g.overlays.top().(*finder)
			require.True(t, open, "the finder should be open")
			assert.Equal(t, [2]any{"kiln", 1}, [2]any{f.field.query(), w.raised})
		})
	})
}
