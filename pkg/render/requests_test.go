package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/single"
)

// fakeWindow is a window that keeps what was asked of it.
type fakeWindow struct {
	minimized, canRaise              bool
	restored, raised, attentionAsked int
	raisedAt                         uint32
}

func (w *fakeWindow) Minimized() bool { return w.minimized }
func (w *fakeWindow) Restore()        { w.restored++; w.minimized = false }
func (w *fakeWindow) Raise(time uint32) bool {
	if w.canRaise {
		w.raised++
		w.raisedAt = time
	}
	return w.canRaise
}
func (w *fakeWindow) RequestAttention() { w.attentionAsked++ }

// fakeShortcut is a desktop's global shortcut that keeps what was
// grabbed, and fires when a test sends it a time.
type fakeShortcut struct {
	fired    chan uint32
	grabbed  *config.Chord
	refuse   error
	released int
}

func (s *fakeShortcut) Grab(c config.Chord) error {
	if s.refuse != nil {
		return s.refuse
	}
	s.grabbed = &c
	return nil
}

func (s *fakeShortcut) Release() error {
	s.grabbed = nil
	s.released++
	return nil
}

func (s *fakeShortcut) Fired() <-chan uint32 { return s.fired }

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

func TestShortcutFiring(t *testing.T) {
	t.Run("when the shortcut fires", func(t *testing.T) {
		w := &fakeWindow{canRaise: true}
		s := &fakeShortcut{fired: make(chan uint32, 1)}
		s.fired <- 4242
		g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1, win: w, shortcut: s}
		require.NoError(t, g.handleKeys())

		t.Run("it should open the finder", func(t *testing.T) {
			assert.True(t, g.finding())
		})

		t.Run("it should raise the window with the key event's time", func(t *testing.T) {
			assert.Equal(t, uint32(4242), w.raisedAt)
		})
	})
}
