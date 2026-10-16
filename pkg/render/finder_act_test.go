package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
)

// recorder is an actor that keeps what it was asked to do.
type recorder struct{ done []city.Action }

func (r *recorder) Do(a city.Action) error {
	r.done = append(r.done, a)
	return nil
}

func TestFinderActs(t *testing.T) {
	live := find.Hit{SessionID: "live", Dir: "/src/kiln", State: state.Working}
	stopped := find.Hit{SessionID: "stopped", Dir: "/src/kiln"}
	gone := find.Hit{SessionID: "gone", Dir: "/src/orchard", Gone: true}
	spaced := find.Hit{SessionID: "spaced", Dir: "/src/my kiln"}
	offMap := find.Hit{SessionID: "elsewhere", Dir: "/src/far"}

	status := func(g *Game) string { return g.notice.Text(timeNow()) }
	dimmed := func(g *Game) map[string]bool {
		out := map[string]bool{}
		for _, b := range g.scene.City().Buildings() {
			out[b.Session.ID] = g.scene.Dimmed(b)
		}
		return out
	}

	t.Run("when the finder has results", func(t *testing.T) {
		t.Run("and a character is typed", func(t *testing.T) {
			g, _ := finderOver(t, 0, live, stopped)

			t.Run("it should dim the map to the sessions found", func(t *testing.T) {
				assert.Equal(t, map[string]bool{"live": false, "stopped": false, "other": true}, dimmed(g))
			})
		})

		t.Run("and Enter is pressed on a live session", func(t *testing.T) {
			g, actor := finderOver(t, 0, live, stopped)
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should attach to it", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionAttach, SessionID: "live"}}, actor.done)
			})

			t.Run("it should close the finder and keep the map dimmed to what was found", func(t *testing.T) {
				assert.Equal(t, [2]any{false, map[string]bool{"live": false, "stopped": false, "other": true}}, [2]any{g.finding(), dimmed(g)})
			})
		})

		t.Run("and Enter is pressed on a stopped session", func(t *testing.T) {
			g, actor := finderOver(t, 1, live, stopped)
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should resume it in a terminal in its own directory", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionResumeIn, SessionID: "stopped", Dir: "/src/kiln"}}, actor.done)
			})
		})

		t.Run("and Enter is pressed on a session whose conversation is gone", func(t *testing.T) {
			g, actor := finderOver(t, 0, gone)
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should say Enter again starts a new session in its directory", func(t *testing.T) {
				assert.Equal(t, [2]any{0, "its conversation was deleted; enter again starts a new session in /src/orchard"}, [2]any{len(actor.done), status(g)})
			})

			t.Run("and Enter is pressed again", func(t *testing.T) {
				pressKey(t, g, ebiten.KeyEnter, false)

				t.Run("it should start a new session there", func(t *testing.T) {
					assert.Equal(t, []city.Action{{Kind: city.ActionStartIn, Dir: "/src/orchard"}}, actor.done)
				})
			})
		})

		t.Run("and Ctrl+Y is pressed", func(t *testing.T) {
			g, actor := finderOver(t, 1, live, stopped)
			pressKey(t, g, ebiten.KeyY, true)

			t.Run("it should copy cd <dir> && claude --resume <id>", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionCopy, Text: "cd /src/kiln && claude --resume stopped"}}, actor.done)
			})

			t.Run("and Ctrl is not held, as when y is typed", func(t *testing.T) {
				g, actor := finderOver(t, 1, live, stopped)
				g.scripted, g.scriptedRune = ebiten.KeyY, 'y'
				require.NoError(t, g.handleKeys())
				g.scripted, g.scriptedRune = -1, 0

				t.Run("it should type it and copy nothing", func(t *testing.T) {
					f, _ := g.overlays.top().(*finder)
					require.NotNil(t, f)
					assert.Equal(t, [2]any{"kilny", 0}, [2]any{f.field.query(), len(actor.done)})
				})
			})

			t.Run("and the directory has a space in it", func(t *testing.T) {
				g, actor := finderOver(t, 0, spaced)
				pressKey(t, g, ebiten.KeyY, true)

				t.Run("it should quote it", func(t *testing.T) {
					assert.Equal(t, []city.Action{{Kind: city.ActionCopy, Text: "cd '/src/my kiln' && claude --resume spaced"}}, actor.done)
				})
			})
		})

		t.Run("and Ctrl+M is pressed", func(t *testing.T) {
			g, _ := finderOver(t, 1, live, stopped)
			pressKey(t, g, ebiten.KeyM, true)

			t.Run("it should close the finder", func(t *testing.T) {
				assert.False(t, g.finding())
			})

			t.Run("it should select the session on the map", func(t *testing.T) {
				require.NotNil(t, g.scene.Selected())
				assert.Equal(t, "stopped", g.scene.Selected().Session.ID)
			})

			t.Run("and Ctrl is not held, as when m is typed", func(t *testing.T) {
				g, _ := finderOver(t, 1, live, stopped)
				g.scripted, g.scriptedRune = ebiten.KeyM, 'm'
				require.NoError(t, g.handleKeys())
				g.scripted, g.scriptedRune = -1, 0
				f, open := g.overlays.top().(*finder)

				t.Run("it should type it", func(t *testing.T) {
					require.True(t, open, "the finder should still be open")
					assert.Equal(t, "kilnm", f.field.query())
				})
			})

			t.Run("and the session is not on the map", func(t *testing.T) {
				g, _ := finderOver(t, 0, offMap)
				pressKey(t, g, ebiten.KeyM, true)

				t.Run("it should say so", func(t *testing.T) {
					assert.Equal(t, "that session is not on the map", status(g))
				})
			})
		})
	})

	t.Run("when the finder has no results", func(t *testing.T) {
		t.Run("and Enter is pressed", func(t *testing.T) {
			g, actor := finderOver(t, 0)
			before := dimmed(g)
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should close the finder and keep the filter", func(t *testing.T) {
				assert.Equal(t, [3]any{false, before, 0}, [3]any{g.finding(), dimmed(g), len(actor.done)})
			})
		})
	})
}

// press presses one key for a tick, with Ctrl held when ctrl is set.
func pressKey(t *testing.T, g *Game, key ebiten.Key, ctrl bool) {
	t.Helper()
	g.scripted, g.scriptedCtrl = key, ctrl
	require.NoError(t, g.handleKeys())
	g.scripted, g.scriptedCtrl = -1, false
}

// finderOver opens the finder over a map holding live, stopped and other,
// types kiln and delivers hits, then moves the selection to row.
func finderOver(t *testing.T, row int, hits ...find.Hit) (*Game, *recorder) {
	t.Helper()
	actor := &recorder{}
	scene := city.NewScene(city.NewLayout())
	scene.Resize(800, 600)
	scene.SetSnapshot(state.Snapshot{Sessions: []state.Session{
		{ID: "live", CWD: "/src/kiln", State: state.Working},
		{ID: "stopped", CWD: "/src/kiln", State: state.Parked},
		{ID: "other", CWD: "/src/kiln", State: state.Waiting},
	}})
	g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1}
	pressKey(t, g, ebiten.KeySlash, false)
	f, ok := g.overlays.top().(*finder)
	require.True(t, ok, "the finder should be open")
	f.now = func() time.Time { return time.Time{} }
	for _, r := range "kiln" {
		g.scriptedRune = r
		require.NoError(t, g.handleKeys())
	}
	g.scriptedRune = 0
	if hits != nil {
		f.deliver("kiln", hits)
		require.NoError(t, g.handleKeys())
	}
	for range row {
		pressKey(t, g, ebiten.KeyArrowDown, false)
	}
	return g, actor
}
