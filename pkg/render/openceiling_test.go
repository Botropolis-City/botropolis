package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
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
		// The dialog can come after the first tick, so the wait takes it
		// as it takes snapshots (item 311).
		t.Run("when a "+kind.name+" has ticked, and no snapshot has come, and its daemon is out of sync", func(t *testing.T) {
			s := city.NewScene(city.NewLayout())
			s.SetOpenOnRead(true)
			g := &Game{scene: s, started: at, ticks: 1, pendingSync: &proto.Mismatch{}, syncChanged: true}
			kind.make(t, g)
			held, err := g.holdUnopened(at.Add(time.Second))

			t.Run("it should not hold", func(t *testing.T) {
				assert.False(t, held)
			})

			t.Run("it should not fail", func(t *testing.T) {
				assert.NoError(t, err)
			})

			t.Run("it should show the dialog", func(t *testing.T) {
				assert.False(t, g.cityShown())
			})
		})

		// Item 311: a recording ticks only once its last tick is painted,
		// so one that ticked before its first snapshot waits for it here.
		t.Run("when a "+kind.name+" has ticked, and nothing has come", func(t *testing.T) {
			s := city.NewScene(city.NewLayout())
			s.SetOpenOnRead(true)
			g := &Game{scene: s, started: at, ticks: 1}
			kind.make(t, g)
			held, err := g.holdUnopened(at.Add(time.Second))

			t.Run("it should hold", func(t *testing.T) {
				assert.True(t, held)
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
			saw     string
		}{
			{"its read moved within the ceiling, with fewer partial hefts", []*state.Snapshot{unread(5, 0), unread(5, 0), unread(4, 0)}, false, ""},
			{"its read moved within the ceiling, with the scan's done risen", []*state.Snapshot{unread(5, 0), unread(5, 10)}, false, ""},
			{"snapshots kept coming with the same partial hefts and scan", []*state.Snapshot{unread(5, 10), unread(5, 10), unread(6, 10), unread(5, 9)}, true,
				"5 sessions' hefts still partial, 10 bytes of the scan done, 3 snapshots since it last moved"},
			{"no snapshot came after the first", []*state.Snapshot{unread(5, 0)}, true,
				"5 sessions' hefts still partial, 0 bytes of the scan done, 0 snapshots since it last moved"},
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

					// Item 249: CI's runner stalls where no machine here
					// does, so its failure says what the capture saw.
					t.Run("it should name the sessions still partial and the snapshots since the read last moved", func(t *testing.T) {
						assert.ErrorContains(t, err, c.saw)
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

// Item 311: a recording ticks only once its last tick is painted, and an
// unopened city paints nothing of the city, so one whose first tick came
// before any snapshot never took the snapshot that would open it.
func TestACaptureWhoseFirstTickComesBeforeAnySnapshot(t *testing.T) {
	at := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	require.NoError(t, err)
	for _, kind := range []struct {
		name string
		make func(t *testing.T, g *Game)
	}{
		{"recording", func(t *testing.T, g *Game) { g.record = t.TempDir() }},
		{"screenshot", func(t *testing.T, g *Game) { g.screenshot = t.TempDir() + "/frame.png" }},
	} {
		// early has the capture tick and draw its opening with no snapshot
		// yet, then has arrive come from the feed.
		early := func(t *testing.T, arrive state.Snapshot) *Game {
			t.Helper()
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return at }, time.UTC)
			s.Resize(800, 600)
			s.SetOpenOnRead(true)
			g := NewGame(s, &recordingActor{}, th, f, nil)
			g.started = at
			kind.make(t, g)
			screen := ebiten.NewImage(800, 600)
			t.Cleanup(screen.Deallocate)
			require.NoError(t, g.Update())
			g.Draw(screen)
			g.Offer(arrive)
			return g
		}

		t.Run("when a "+kind.name+" ticks and draws its opening before any snapshot, and a read one then arrives", func(t *testing.T) {
			g := early(t, state.Snapshot{At: at, HeftsRead: true})
			require.NoError(t, g.Update())
			require.NoError(t, g.Update())

			t.Run("it should open the city", func(t *testing.T) {
				assert.True(t, g.scene.Opened())
			})

			t.Run("it should start the capture", func(t *testing.T) {
				assert.Equal(t, 1, g.shotFrames)
			})
		})

		t.Run("when a "+kind.name+" ticks and draws its opening before any snapshot, and an unread one arrives, and it has waited past the ceiling", func(t *testing.T) {
			g := early(t, state.Snapshot{At: at})
			_, err := g.holdUnopened(at.Add(openCeiling + time.Second))

			t.Run("it should fail the capture, naming the history not read within the ceiling", func(t *testing.T) {
				assert.ErrorContains(t, err, "the history was not read within 30s")
			})
		})
	}
}
