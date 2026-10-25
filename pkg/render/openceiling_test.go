package render

import (
	"fmt"
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

// Item 249: under load a read can be slow and still advancing, so the
// harness that runs acceptance captures has a capture fail only once the
// read has made no progress for the ceiling.
func TestACaptureUnderTheStallCeiling(t *testing.T) {
	at := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	unread := func(partial int, done int64) *state.Snapshot {
		s := &state.Snapshot{At: at, Scan: &state.ScanProgress{Done: done, Known: 1 << 30}}
		for range partial {
			s.Sessions = append(s.Sessions, state.Session{HeftPartial: true})
		}
		return s
	}
	for _, kind := range []struct {
		name string
		make func(t *testing.T, g *Game)
	}{
		{"recording", func(t *testing.T, g *Game) { g.record = t.TempDir() }},
		{"screenshot", func(t *testing.T, g *Game) { g.screenshot = t.TempDir() + "/frame.png" }},
	} {
		// wait has the city take the first snapshot as it opens, offers
		// the nth n seconds after the start, then holds at last.
		wait := func(t *testing.T, offered []*state.Snapshot, last time.Duration) error {
			t.Helper()
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return at }, time.UTC)
			s.Resize(800, 600)
			s.SetOpenOnRead(true)
			s.SetSnapshot(*offered[0])
			g := &Game{scene: s, started: at, ticks: 1, ceiling: stallCeiling}
			kind.make(t, g)
			for i, snapshot := range offered {
				g.pending = snapshot
				if _, err := g.holdUnopened(at.Add(time.Duration(i+1) * time.Second)); err != nil {
					return err
				}
			}
			_, err := g.holdUnopened(at.Add(last))
			return err
		}

		for _, c := range []struct {
			name    string
			offered []*state.Snapshot
			fails   bool
		}{
			{"its read moved within the ceiling, with fewer partial hefts", []*state.Snapshot{unread(5, 0), unread(5, 0), unread(4, 0)}, false},
			{"its read moved within the ceiling, with the scan's done risen", []*state.Snapshot{unread(5, 0), unread(5, 10)}, false},
			{"snapshots kept coming with the same partial hefts and scan", []*state.Snapshot{unread(5, 10), unread(5, 10), unread(6, 10), unread(5, 9)}, true},
			{"no snapshot came after the first", []*state.Snapshot{unread(5, 0)}, true},
		} {
			t.Run("when a "+kind.name+" holds an unread snapshot, and it has waited past the ceiling, and "+c.name, func(t *testing.T) {
				last := openCeiling + 20*time.Second
				if !c.fails {
					last = time.Duration(len(c.offered))*time.Second + openCeiling - time.Second
				}
				err := wait(t, c.offered, last)

				if c.fails {
					t.Run("it should fail the capture, naming the read making no progress for the ceiling", func(t *testing.T) {
						assert.ErrorContains(t, err, "the history was not read: the read made no progress for 30s")
					})
					return
				}
				t.Run("it should not fail", func(t *testing.T) {
					assert.NoError(t, err)
				})
			})
		}
	}
}

func TestOpenCeilingMode(t *testing.T) {
	for _, c := range []struct {
		value string
		want  ceilingMode
	}{
		{"stall", stallCeiling},
		{"", totalCeiling},
		{"total", totalCeiling},
		{"Stall ", totalCeiling},
	} {
		t.Run(fmt.Sprintf("when BOTROPOLIS_FRAME_OPEN_CEILING is %q, it should choose %s", c.value, c.want), func(t *testing.T) {
			assert.Equal(t, c.want, openCeilingMode(c.value))
		})
	}
}
