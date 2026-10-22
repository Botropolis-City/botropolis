package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 202: a recording opens the city once its history is read, and
// opens it anyway on what it holds once it has waited openCeiling, so a
// history that never reads cannot hold a film or a frame test for ever.
func TestARecordingOpensAtTheCeiling(t *testing.T) {
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	recording := func(t *testing.T) *Game {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		s.SetOpenOnRead(true)
		s.SetSnapshot(state.Snapshot{At: at})
		return &Game{scene: s, record: t.TempDir(), started: at}
	}

	// A recording's clock waits for each tick to be painted, and an
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
		t.Run("when a recording that has ticked once waits unopened, and "+c.name+" arrives", func(t *testing.T) {
			g := recording(t)
			g.ticks = 1
			g.pending = &state.Snapshot{At: at, HeftsRead: c.read}
			held := g.holdUnopened(at.Add(time.Second))

			t.Run("it should say whether to hold", func(t *testing.T) {
				assert.Equal(t, c.held, held)
			})

			t.Run("it should not tick", func(t *testing.T) {
				assert.Equal(t, uint64(1), g.ticks)
			})
		})
	}

	// A daemon of another release sends nothing, and its dialog is what
	// the frame shows (item 152), so there is no read to wait for.
	t.Run("when a recording has ticked and no snapshot has come", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.SetOpenOnRead(true)
		g := &Game{scene: s, record: t.TempDir(), started: at, ticks: 1}

		t.Run("it should not hold", func(t *testing.T) {
			assert.False(t, g.holdUnopened(at.Add(time.Second)))
		})
	})

	t.Run("when a recording holds an unread snapshot, and it has waited less than the ceiling", func(t *testing.T) {
		g := recording(t)
		g.openAtCeiling(at.Add(openCeiling - time.Second))

		t.Run("it should not open", func(t *testing.T) {
			assert.False(t, g.scene.Opened())
		})
	})

	t.Run("when a recording holds an unread snapshot, and it has waited the ceiling", func(t *testing.T) {
		g := recording(t)
		g.openAtCeiling(at.Add(openCeiling))

		t.Run("it should open", func(t *testing.T) {
			assert.True(t, g.scene.Opened())
		})

		t.Run("it should start the shot", func(t *testing.T) {
			assert.Positive(t, g.shotFrames)
		})

		t.Run("it should say it opened early", func(t *testing.T) {
			assert.True(t, g.openedEarly)
		})
	})
}
