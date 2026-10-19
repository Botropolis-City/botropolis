package render

import (
	"context"
	"errors"
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

// Item 152: a daemon out of sync with the map stops it, with a dialog
// that offers to exit or to restart whichever end is behind.
func TestSyncDialog(t *testing.T) {
	ours := proto.Peer{Release: "0.2.3", Protocol: 1}
	older := &proto.Mismatch{Daemon: proto.Peer{Release: "0.2.2", Protocol: 1}, Ours: ours}
	newer := &proto.Mismatch{Daemon: proto.Peer{Release: "0.2.4", Protocol: 1}, Ours: ours}
	th := ui.NewTheme(1)
	faces, err := newFaces(th)
	require.NoError(t, err)

	type harness struct {
		g        *Game
		restarts int
		fail     error
	}
	newHarness := func(m *proto.Mismatch) *harness {
		h := &harness{}
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		h.g = &Game{scene: s, theme: th, faces: faces, scripted: -1, clicked: -1,
			dispatch: func(run func()) { run() },
			restartDaemon: func(context.Context) error {
				h.restarts++
				return h.fail
			},
		}
		h.g.OutOfSync(m)
		h.g.takeSync()
		return h
	}
	press := func(g *Game, key ebiten.Key) error {
		g.scripted = key
		defer func() { g.scripted = -1 }()
		return g.syncInput(city.Point{X: -1, Y: -1}, false)
	}
	click := func(g *Game, label string) error {
		for _, b := range g.syncButtons(800, 600) {
			if b.Label.Text == label {
				return g.syncInput(b.Rect.Center(), true)
			}
		}
		return errors.New("no button " + label)
	}

	t.Run("when the daemon is older than the city", func(t *testing.T) {
		h := newHarness(older)
		w := h.g.sync.wording()

		t.Run("it should say they are out of sync", func(t *testing.T) {
			assert.Equal(t, "Botropolis is out of sync with its daemon", w.Title)
		})

		t.Run("it should say to restart the daemon", func(t *testing.T) {
			assert.Equal(t, []string{"The Botropolis city and its daemon must be running the same version. Please restart the daemon to continue."}, w.Lines)
		})

		t.Run("it should list both versions", func(t *testing.T) {
			assert.Equal(t, [][2]string{{"Botropolis daemon version", "0.2.2"}, {"Botropolis city version", "0.2.3"}}, w.Versions)
		})

		t.Run("it should offer Exit and Restart Daemon", func(t *testing.T) {
			assert.Equal(t, []string{"Exit", "Restart Daemon"}, w.Buttons)
		})

		t.Run("it should draw no city", func(t *testing.T) {
			assert.False(t, h.g.cityShown())
		})

		for _, key := range []ebiten.Key{ebiten.KeyEscape, ebiten.KeyQ} {
			t.Run("and "+key.String()+" is pressed", func(t *testing.T) {
				h := newHarness(older)

				t.Run("it should end the game", func(t *testing.T) {
					assert.Equal(t, ebiten.Termination, press(h.g, key))
				})
			})
		}

		t.Run("and Exit is clicked", func(t *testing.T) {
			h := newHarness(older)

			t.Run("it should end the game", func(t *testing.T) {
				assert.Equal(t, ebiten.Termination, click(h.g, "Exit"))
			})
		})

		t.Run("and Restart Daemon is clicked", func(t *testing.T) {
			h := newHarness(older)
			dispatched := 0
			h.g.dispatch = func(func()) { dispatched++ }
			require.NoError(t, click(h.g, "Restart Daemon"))
			require.NoError(t, press(h.g, ebiten.KeyEnter))

			t.Run("it should start one restart, however often it is asked", func(t *testing.T) {
				assert.Equal(t, 1, dispatched)
			})

			t.Run("it should say it is restarting", func(t *testing.T) {
				assert.Equal(t, "Restarting the daemon…", h.g.sync.wording().Status)
			})

			t.Run("it should offer nothing to press while it restarts", func(t *testing.T) {
				assert.Empty(t, h.g.sync.wording().Buttons)
			})
		})

		for _, key := range []ebiten.Key{ebiten.KeyEnter, ebiten.KeyR} {
			t.Run("and "+key.String()+" is pressed", func(t *testing.T) {
				h := newHarness(older)
				require.NoError(t, press(h.g, key))

				t.Run("it should restart the daemon once", func(t *testing.T) {
					assert.Equal(t, 1, h.restarts)
				})
			})
		}

		t.Run("and the restart fails", func(t *testing.T) {
			h := newHarness(older)
			h.fail = errors.New("systemctl --user restart botropolisd.service: Unit botropolisd.service not found.")
			require.NoError(t, press(h.g, ebiten.KeyEnter))
			h.g.takeSync()

			t.Run("it should say why", func(t *testing.T) {
				assert.Equal(t, "The restart failed: systemctl --user restart botropolisd.service: Unit botropolisd.service not found.", h.g.sync.wording().Status)
			})

			t.Run("it should offer both buttons again", func(t *testing.T) {
				assert.Equal(t, []string{"Exit", "Restart Daemon"}, h.g.sync.wording().Buttons)
			})

			t.Run("it should keep the city hidden", func(t *testing.T) {
				assert.False(t, h.g.cityShown())
			})
		})

		t.Run("and a daemon at this release answers", func(t *testing.T) {
			h := newHarness(older)
			h.g.Offer(state.Snapshot{At: time.Now()})
			h.g.takeSync()

			t.Run("it should close the dialog", func(t *testing.T) {
				assert.Nil(t, h.g.sync)
			})

			t.Run("it should show the city", func(t *testing.T) {
				assert.True(t, h.g.cityShown())
			})
		})
	})

	t.Run("when the daemon is newer than the city", func(t *testing.T) {
		h := newHarness(newer)

		t.Run("it should say to restart the city", func(t *testing.T) {
			assert.Equal(t, []string{"The Botropolis city and its daemon must be running the same version. Please restart the city to continue."}, h.g.sync.wording().Lines)
		})

		t.Run("it should list both versions", func(t *testing.T) {
			assert.Equal(t, [][2]string{{"Botropolis daemon version", "0.2.4"}, {"Botropolis city version", "0.2.3"}}, h.g.sync.wording().Versions)
		})

		t.Run("it should offer Exit and Restart City", func(t *testing.T) {
			assert.Equal(t, []string{"Exit", "Restart City"}, h.g.sync.wording().Buttons)
		})

		t.Run("and Restart City is clicked", func(t *testing.T) {
			h := newHarness(newer)

			t.Run("it should end the game to restart it", func(t *testing.T) {
				assert.ErrorIs(t, click(h.g, "Restart City"), ErrRestart)
			})

			t.Run("it should not touch the daemon", func(t *testing.T) {
				assert.Zero(t, h.restarts)
			})
		})
	})
}
