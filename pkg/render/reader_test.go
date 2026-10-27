package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
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
	// laidOut lays the pane out as a frame would, with seven-pixel type,
	// since a test has no window to draw it in.
	laidOut := func(g *Game) {
		measure := func(s string, _ ui.Size) (float64, float64) { return 7 * float64(len([]rune(s))), 14 }
		styled := func(s string, size ui.Size, _ ui.SpanStyle) (float64, float64) { return measure(s, size) }
		pane(g).layout(g, ui.NewTheme(1), 1280, 800, measure, styled)
	}
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

	t.Run("when a page of the conversation is read", func(t *testing.T) {
		page := claude.Conversation{Messages: []claude.Message{
			{From: claude.FromYou, Text: "make it **bold**"},
			{From: claude.FromTool, Text: "echo `date`"},
			{From: claude.FromClaude, Text: "it is **bold**"},
		}}
		g, _, _ := opened(t, "live", page, nil)
		turns := pane(g).input(g).Turns
		require.Len(t, turns, 3)

		t.Run("it should hand the layout Claude's reply already read as Markdown", func(t *testing.T) {
			assert.Equal(t, ui.ParseMarkdown("it is **bold**"), turns[2].Blocks)
		})

		t.Run("it should leave your prompts and tool calls unread", func(t *testing.T) {
			assert.Equal(t, [2]bool{true, true}, [2]bool{turns[0].Blocks == nil, turns[1].Blocks == nil})
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
			{"live", city.Action{Kind: city.ActionAttach, SessionID: "live", Dir: "/src/kiln"}},
			{"parked", city.Action{Kind: city.ActionResume, SessionID: "parked"}},
		} {
			t.Run("and Enter is pressed on a "+c.id+" session", func(t *testing.T) {
				g, actor, _ := opened(t, c.id, five, nil)
				pressKey(t, g, ebiten.KeyEnter, false)
				g.settle()

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

	t.Run("when the selection's transcript is gone", func(t *testing.T) {
		gone := func(t *testing.T, kept []claude.Prompt) (*Game, *recorder) {
			t.Helper()
			g, actor, _ := setup(t, "live", claude.Conversation{}, errTranscriptGone)
			g.readPrompts = func(id string) ([]claude.Prompt, error) {
				require.Equal(t, "live", id)
				return kept, nil
			}
			pressKey(t, g, ebiten.KeyO, false)
			require.True(t, g.reading(), "the pane should be open")
			return g, actor
		}
		g, actor := gone(t, []claude.Prompt{{Text: "why does the kiln overheat"}, {Text: "tighten it"}})

		t.Run("it should show the prompts history.jsonl has for it", func(t *testing.T) {
			assert.Equal(t, []ui.ReaderTurn{{From: ui.FromYou, Text: "why does the kiln overheat"}, {From: ui.FromYou, Text: "tighten it"}}, pane(g).input(g).Turns)
		})

		t.Run("and Enter is pressed", func(t *testing.T) {
			pressKey(t, g, ebiten.KeyEnter, false)
			g.settle()

			t.Run("it should start a new session in its directory", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionNew, Dir: "/src/kiln"}}, actor.done)
			})
		})

		t.Run("and a kept prompt's copy button is pressed", func(t *testing.T) {
			long := "convert the celadon glaze recipe from cone 10 to cone 6, " + strings.Repeat("keeping the iron oxide at two percent, ", 8)
			g, actor := gone(t, []claude.Prompt{{Text: "why does the kiln overheat"}, {Text: long}})
			laidOut(g)
			r := pane(g)
			require.Len(t, r.panel.Copies, 2)
			stays := g.clickOverlay(r.panel.Copies[1].Button.Rect.Center()) && g.reading()
			g.settle()

			t.Run("the clipboard should hold that prompt's text exactly, even when it wraps over several lines", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionCopy, Text: long}}, actor.done)
			})

			t.Run("it should keep the pane open", func(t *testing.T) {
				assert.True(t, stays)
			})
		})

		t.Run("and the pointer clicks inside the panel but on no button", func(t *testing.T) {
			g, actor := gone(t, []claude.Prompt{{Text: "why does the kiln overheat"}})
			laidOut(g)
			g.clickOverlay(pane(g).panel.Title.At)

			t.Run("it should keep the pane open", func(t *testing.T) {
				assert.True(t, g.reading())
			})

			t.Run("it should copy nothing", func(t *testing.T) {
				assert.Empty(t, actor.done)
			})
		})

		t.Run("and history.jsonl has nothing for it", func(t *testing.T) {
			bare, _ := gone(t, nil)

			t.Run("it should say no prompts were kept", func(t *testing.T) {
				assert.Equal(t, "Claude Code kept no prompts for it", pane(bare).input(bare).Note)
			})
		})
	})

	t.Run("when the session is working and a call is pending", func(t *testing.T) {
		g, _, _ := opened(t, "live", pending, nil)

		t.Run("it should not lead with it", func(t *testing.T) {
			assert.Empty(t, pane(g).input(g).Approval)
		})
	})
}
