package render

import (
	"errors"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 83. The reading pane is the overlay stack's second tenant: o, or
// read in the selection's summary, opens it over the city, and it reads
// a page of the transcript at a time.

type pageRead struct {
	id          string
	skip, count int
}

func TestReadingPane(t *testing.T) {
	pending := claude.Conversation{
		Messages: []claude.Message{{From: claude.FromYou, Text: "rebuild the kiln"}, {From: claude.FromTool, Text: "rm -rf build/"}},
		Pending:  []claude.PendingCall{{Tool: "Bash", Input: "rm -rf build/"}},
	}
	// setup is a map holding a live, a parked and a blocked session, with
	// id selected, reading pages through a fake that answers with conv.
	setup := func(t *testing.T, id string, conv claude.Conversation, err error) (*Game, *recorder, *[]pageRead) {
		t.Helper()
		actor := &recorder{}
		scene := city.NewScene(city.NewLayout())
		scene.Resize(800, 600)
		scene.SetSnapshot(state.Snapshot{Sessions: []state.Session{
			{ID: "live", CWD: "/src/kiln", State: state.Working},
			{ID: "parked", CWD: "/src/kiln", State: state.Resting},
			{ID: "blocked", CWD: "/src/kiln", State: state.NeedsYou, Note: "Claude needs your permission to use Bash"},
		}})
		if id != "" {
			require.True(t, scene.Select(id))
		}
		reads := &[]pageRead{}
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1,
			dispatch: func(run func()) { run() },
			readConversation: func(id string, skip, count int) (claude.Conversation, error) {
				*reads = append(*reads, pageRead{id, skip, count})
				return conv, err
			},
		}
		return g, actor, reads
	}
	opened := func(t *testing.T, id string, conv claude.Conversation, err error) (*Game, *recorder, *[]pageRead) {
		t.Helper()
		g, actor, reads := setup(t, id, conv, err)
		pressKey(t, g, ebiten.KeyO, false)
		require.True(t, g.reading(), "the pane should be open")
		return g, actor, reads
	}
	pane := func(g *Game) *reader { r, _ := g.overlays.top().(*reader); return r }
	five := claude.Conversation{Messages: make([]claude.Message, 5)}

	t.Run("when a session is selected and o is pressed", func(t *testing.T) {
		g, _, reads := opened(t, "live", five, nil)

		t.Run("it should open the pane", func(t *testing.T) {
			assert.True(t, g.reading())
		})

		t.Run("it should read that session's transcript", func(t *testing.T) {
			require.NotEmpty(t, *reads)
			assert.Equal(t, "live", (*reads)[0].id)
		})
	})

	t.Run("when nothing is selected and o is pressed", func(t *testing.T) {
		g, _, _ := setup(t, "", five, nil)
		pressKey(t, g, ebiten.KeyO, false)

		t.Run("it should say to select a building first", func(t *testing.T) {
			assert.Equal(t, "select a building first", g.notice.Text(timeNow()))
		})
	})

	t.Run("when the summary's read is clicked", func(t *testing.T) {
		g, _, _ := setup(t, "live", five, nil)
		action, _ := g.scene.Act(city.ActionRead)
		g.act(action)

		t.Run("it should open the pane", func(t *testing.T) {
			assert.True(t, g.reading())
		})
	})

	t.Run("when the pane is open", func(t *testing.T) {
		t.Run("and a key the map binds is pressed (t, x, s, Tab)", func(t *testing.T) {
			g, _, _ := opened(t, "live", five, nil)
			for _, key := range []ebiten.Key{ebiten.KeyT, ebiten.KeyX, ebiten.KeyS, ebiten.KeyTab} {
				pressKey(t, g, key, false)
			}

			t.Run("it should not reach the map", func(t *testing.T) {
				assert.Equal(t, [3]bool{}, [3]bool{g.timeline, g.breakdown, g.settingsOpen})
			})
		})

		t.Run("and Esc is pressed", func(t *testing.T) {
			g, _, _ := opened(t, "live", five, nil)
			pressKey(t, g, ebiten.KeyEscape, false)

			t.Run("it should close the pane", func(t *testing.T) {
				assert.False(t, g.reading())
			})

			t.Run("it should keep the selection", func(t *testing.T) {
				require.NotNil(t, g.scene.Selected())
				assert.Equal(t, "live", g.scene.Selected().Session.ID)
			})

			t.Run("it should keep nothing it read", func(t *testing.T) {
				assert.Empty(t, g.overlays)
			})
		})

		t.Run("and the pointer clicks outside the panel", func(t *testing.T) {
			g, _, _ := opened(t, "live", five, nil)
			g.clickOverlay(city.Point{X: 1, Y: 1})

			t.Run("it should close the pane", func(t *testing.T) {
				assert.False(t, g.reading())
			})
		})

		t.Run("and PgUp is pressed", func(t *testing.T) {
			g, _, reads := opened(t, "live", five, nil)
			pane(g).shown = 5
			pressKey(t, g, ebiten.KeyPageUp, false)

			t.Run("it should read the page before", func(t *testing.T) {
				assert.Equal(t, 5, (*reads)[len(*reads)-1].skip)
			})

			t.Run("and PgDn is pressed after it", func(t *testing.T) {
				pressKey(t, g, ebiten.KeyPageDown, false)

				t.Run("it should come back a page", func(t *testing.T) {
					assert.Equal(t, 0, (*reads)[len(*reads)-1].skip)
				})
			})
		})

		t.Run("and PgDn is pressed at the newest", func(t *testing.T) {
			g, _, reads := opened(t, "live", five, nil)
			before := len(*reads)
			pressKey(t, g, ebiten.KeyPageDown, false)

			t.Run("it should stay", func(t *testing.T) {
				assert.Len(t, *reads, before)
			})
		})

		t.Run("and PgUp is pressed at the start", func(t *testing.T) {
			g, _, reads := opened(t, "live", claude.Conversation{Messages: five.Messages, AtStart: true}, nil)
			pane(g).shown = 5
			before := len(*reads)
			pressKey(t, g, ebiten.KeyPageUp, false)

			t.Run("it should stay", func(t *testing.T) {
				assert.Len(t, *reads, before)
			})
		})

		for _, c := range []struct {
			id   string
			want city.Action
		}{
			{"live", city.Action{Kind: city.ActionAttach, SessionID: "live"}},
			{"parked", city.Action{Kind: city.ActionResume, SessionID: "parked"}},
		} {
			t.Run("and Enter is pressed on a "+c.id+" session", func(t *testing.T) {
				g, actor, _ := opened(t, c.id, five, nil)
				pressKey(t, g, ebiten.KeyEnter, false)

				t.Run("it should close the pane", func(t *testing.T) {
					assert.False(t, g.reading())
				})

				t.Run("it should attach a live session, or resume a parked one", func(t *testing.T) {
					assert.Equal(t, []city.Action{c.want}, actor.done)
				})
			})
		}
	})

	t.Run("when the read fails", func(t *testing.T) {
		g, _, _ := opened(t, "live", claude.Conversation{}, errors.New("transcript unreadable"))

		t.Run("it should say why in the pane", func(t *testing.T) {
			assert.Equal(t, "transcript unreadable", pane(g).input(g).Err)
		})
	})

	t.Run("when the session needs you and a call is pending", func(t *testing.T) {
		g, _, _ := opened(t, "blocked", pending, nil)
		in := pane(g).input(g)

		t.Run("it should lead with the approval", func(t *testing.T) {
			assert.Equal(t, []string{"rm -rf build/"}, in.Approval)
		})

		t.Run("it should carry the hook's note", func(t *testing.T) {
			assert.Equal(t, "Claude needs your permission to use Bash", in.Note)
		})
	})

	t.Run("when the session is working and a call is pending", func(t *testing.T) {
		g, _, _ := opened(t, "live", pending, nil)

		t.Run("it should not lead with it", func(t *testing.T) {
			assert.Empty(t, pane(g).input(g).Approval)
		})
	})
}
