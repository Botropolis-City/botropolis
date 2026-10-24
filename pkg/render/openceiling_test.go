package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 202: a capture opens the city once its history is read. Item 240:
// one that has waited openCeiling on a history still unread fails,
// naming why, since a frame taken from a partial read is wrong without
// saying so.
func TestACaptureFailsAtTheCeiling(t *testing.T) {
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	for _, kind := range []struct {
		name string
		make func(t *testing.T, g *Game)
	}{
		{"recording", func(t *testing.T, g *Game) { g.record = t.TempDir() }},
		{"screenshot", func(t *testing.T, g *Game) { g.screenshot = t.TempDir() + "/frame.png" }},
	} {
		capture := func(t *testing.T) *Game {
			t.Helper()
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return at }, time.UTC)
			s.Resize(800, 600)
			s.SetOpenOnRead(true)
			s.SetSnapshot(state.Snapshot{At: at})
			g := &Game{scene: s, started: at}
			kind.make(t, g)
			return g
		}

		// A capture's clock waits for each tick to be painted, and an
		// unopened city paints nothing of it, so the wait takes snapshots
		// without ticking, as the wait for the atlas does.
		for _, c := range []struct {
			name string
			read bool
			held bool
		}{
			{"an unread one", false, true},
			{"a read one", true, false},
		} {
			t.Run("when a "+kind.name+" that has ticked once waits unopened, and "+c.name+" arrives", func(t *testing.T) {
				g := capture(t)
				g.ticks = 1
				g.pending = &state.Snapshot{At: at, HeftsRead: c.read}
				held, err := g.holdUnopened(at.Add(time.Second))

				t.Run("it should say whether to hold", func(t *testing.T) {
					assert.Equal(t, c.held, held)
				})

				t.Run("it should not fail", func(t *testing.T) {
					assert.NoError(t, err)
				})

				t.Run("it should not tick", func(t *testing.T) {
					assert.Equal(t, uint64(1), g.ticks)
				})
			})
		}

		// A daemon of another release sends nothing, and its dialog is
		// what the frame shows (item 152), so there is no read to wait for.
		t.Run("when a "+kind.name+" has ticked and no snapshot has come", func(t *testing.T) {
			s := city.NewScene(city.NewLayout())
			s.SetOpenOnRead(true)
			g := &Game{scene: s, started: at, ticks: 1}
			kind.make(t, g)
			held, err := g.holdUnopened(at.Add(time.Second))

			t.Run("it should not hold", func(t *testing.T) {
				assert.False(t, held)
			})

			t.Run("it should not fail", func(t *testing.T) {
				assert.NoError(t, err)
			})
		})

		t.Run("when a "+kind.name+" holds an unread snapshot, and it has waited less than the ceiling", func(t *testing.T) {
			g := capture(t)
			g.ticks = 1
			_, err := g.holdUnopened(at.Add(openCeiling - time.Second))

			t.Run("it should not open", func(t *testing.T) {
				assert.False(t, g.scene.Opened())
			})

			t.Run("it should not fail", func(t *testing.T) {
				assert.NoError(t, err)
			})
		})

		t.Run("when a "+kind.name+" holds an unread snapshot, and it has waited the ceiling", func(t *testing.T) {
			g := capture(t)
			g.ticks = 1
			_, err := g.holdUnopened(at.Add(openCeiling))

			t.Run("it should not open", func(t *testing.T) {
				assert.False(t, g.scene.Opened())
			})

			t.Run("it should fail the capture, naming the history not read within the ceiling", func(t *testing.T) {
				assert.ErrorContains(t, err, "the history was not read within 30s")
			})
		})
	}
}
