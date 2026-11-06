package render

import (
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350, slice 6a: every list panel speaks one vocabulary. Each is
// opened over a city with at least three rows in it, and each key of the
// vocabulary is pressed in it.
func TestEveryListSpeaksTheVocabulary(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	newGame := func(t *testing.T) *Game {
		t.Helper()
		var sessions []state.Session
		for _, s := range []struct{ id, dir, group string }{
			{"a", "/w/lib", "g1"}, {"b", "/w/api", "g1"}, {"c", "/w/web", "g1"}, {"d", "/w/doc", "g2"}, {"e", "/w/ops", "g3"},
		} {
			sessions = append(sessions, state.Session{ID: s.id, Title: "Title " + s.id, CWD: s.dir, State: state.NeedsYou, Kind: claude.KindBackground,
				Group: s.group, StartedAt: now.Add(-time.Hour), LastActivity: now.Add(-time.Minute)})
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetInstant(true)
		s.Resize(1600, 1200)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: sessions, Groups: []state.Group{
			{ID: "g1", Name: "Lib, api and web", Members: []string{"a", "b", "c"}},
			{ID: "g2", Name: "Docs", Members: []string{"d"}},
			{ID: "g3", Name: "Ops", Members: []string{"e"}},
		}})
		s.AddEvents([]events.Event{
			{At: now.Add(-3 * time.Minute), Kind: events.NeedsYou, SessionID: "a", Title: "Title a"},
			{At: now.Add(-2 * time.Minute), Kind: events.NeedsYou, SessionID: "b", Title: "Title b"},
			{At: now.Add(-time.Minute), Kind: events.NeedsYou, SessionID: "c", Title: "Title c"},
		})
		return &Game{scene: s, actor: &recordingActor{}, scripted: -1, clicked: -1}
	}
	top := func(g *Game) overlay { return g.overlays.top() }

	type panel struct {
		name   string
		open   func(t *testing.T, g *Game)
		focus  func(g *Game) int
		chosen func(g *Game) bool
		closed func(g *Game) bool
		// verb is a list whose Enter is the panel's own: there 1-9 only
		// focus, and Enter is pinned by the panel's own tests.
		verb bool
		// docked is a list on the map, where 1-6 stay the lenses'.
		docked bool
		// filterTo is what / and the letters after it narrow the list
		// to: one row, which filtered says Enter, or the focus, landed on.
		filterTo string
		filtered func(g *Game) bool
	}
	actedOn := func(g *Game, kind city.ActionKind, group string) bool {
		for _, a := range g.actor.(*recordingActor).done {
			if a.Kind == kind && a.Group == group {
				return true
			}
		}
		return false
	}
	selected := func(g *Game, id string) bool {
		b := g.scene.Selected()
		return b != nil && b.Session.ID == id
	}
	panels := []panel{
		{name: "the group picker",
			open:     func(t *testing.T, g *Game) { g.openPicker("a") },
			focus:    func(g *Game) int { return g.picker().selected },
			chosen:   func(g *Game) bool { return g.picker() == nil || g.picker().naming },
			closed:   func(g *Game) bool { _, ok := top(g).(*picker); return !ok },
			filterTo: "ops", filtered: func(g *Game) bool { return actedOn(g, city.ActionMove, "g3") }},
		{name: "the clinic",
			open:     func(t *testing.T, g *Game) { g.openClinic() },
			focus:    func(g *Game) int { return top(g).(*clinic).selected },
			chosen:   func(g *Game) bool { _, open := top(g).(*clinic); return !open && g.scene.Selected() != nil },
			closed:   func(g *Game) bool { _, ok := top(g).(*clinic); return !ok },
			filterTo: "title c", filtered: func(g *Game) bool { return selected(g, "c") }},
		{name: "the groups board",
			open:     func(t *testing.T, g *Game) { g.openBoard() },
			focus:    func(g *Game) int { return g.board().selected },
			chosen:   func(g *Game) bool { return g.scene.FocusedGroup() != "" },
			closed:   func(g *Game) bool { return g.board() == nil },
			filterTo: "ops", filtered: func(g *Game) bool { return g.scene.FocusedGroup() == "g3" }},
		{name: "the timeline",
			open:     func(t *testing.T, g *Game) { g.timeline = true },
			focus:    func(g *Game) int { return g.cursor },
			chosen:   func(g *Game) bool { return !g.timeline && g.scene.Selected() != nil },
			closed:   func(g *Game) bool { return !g.timeline },
			filterTo: "title b", filtered: func(g *Game) bool { return selected(g, "b") }},
		{name: "the settings",
			open: func(t *testing.T, g *Game) {
				g.settings = ui.Settings{Items: []ui.Setting{
					{Key: "one", Label: "one", Options: []string{"x", "y"}, Value: "x"},
					{Key: "two", Label: "two", Options: []string{"x", "y"}, Value: "x"},
					{Key: "three", Label: "three", Options: []string{"x", "y"}, Value: "x"},
				}}
				g.settingsOpen = true
			},
			focus:    func(g *Game) int { return g.settings.Cursor },
			chosen:   func(g *Game) bool { return g.settings.Items[g.settings.Cursor].Value == "y" },
			closed:   func(g *Game) bool { return !g.settingsOpen },
			filterTo: "three", filtered: func(g *Game) bool { return g.settings.Items[2].Value == "y" && g.settings.Items[0].Value == "x" }},
		{name: "the journal",
			open: func(t *testing.T, g *Game) {
				entries := []city.JournalEntry{{Title: "one", Fly: "a"}, {Title: "two", Fly: "b"}, {Title: "three", Fly: "c"}}
				g.overlays.push(&journal{entries: entries, filled: make([]bool, len(entries))})
			},
			focus:    func(g *Game) int { return top(g).(*journal).selected },
			chosen:   func(g *Game) bool { _, open := top(g).(*journal); return !open && g.scene.Selected() != nil },
			closed:   func(g *Game) bool { _, ok := top(g).(*journal); return !ok },
			filterTo: "three", filtered: func(g *Game) bool { return selected(g, "c") }},
		{name: "the Keys panel",
			open:     func(t *testing.T, g *Game) { g.openKeysPanel() },
			focus:    func(g *Game) int { return top(g).(*keysPanel).focus },
			chosen:   func(g *Game) bool { return top(g).(*keysPanel).recording },
			closed:   func(g *Game) bool { _, ok := top(g).(*keysPanel); return !ok },
			filterTo: "timeline", filtered: func(g *Game) bool {
				p := top(g).(*keysPanel)
				return p.recording && p.rows[p.focus].name == "timeline"
			}},
		{name: "the dive",
			open: func(t *testing.T, g *Game) {
				require.True(t, g.scene.FocusGroup("g1"))
				g.scene.StepDive(1)
				require.GreaterOrEqual(t, len(g.scene.Dive()), 3)
			},
			focus:    func(g *Game) int { return g.scene.DiveStep() },
			chosen:   func(g *Game) bool { _, ok := top(g).(*reader); return ok },
			closed:   func(g *Game) bool { return g.scene.FocusedGroup() == "" },
			docked:   true,
			filterTo: "web", filtered: func(g *Game) bool {
				rows := g.scene.Dive()
				step := g.scene.DiveStep()
				_, reading := top(g).(*reader)
				return reading && step >= 0 && step < len(rows) && strings.Contains(rows[step].Site, "web")
			}},
		{name: "close-up",
			open: func(t *testing.T, g *Game) {
				pressShift(t, g, ebiten.KeyQ)
				_, ok := top(g).(*closeUp)
				require.True(t, ok)
			},
			focus:    func(g *Game) int { return top(g).(*closeUp).selected },
			closed:   func(g *Game) bool { _, ok := top(g).(*closeUp); return !ok },
			verb:     true,
			filterTo: "title c", filtered: func(g *Game) bool {
				p := top(g).(*closeUp)
				return p.rows[p.selected].ID == "c"
			}},
		{name: "the tidy-up walk",
			open: func(t *testing.T, g *Game) {
				g.overlays.push(&walk{cards: []string{"g1"}, review: group.Review{Rows: []group.ReviewRow{{ID: "g1", Name: "Lib, api and web",
					Members: []group.ReviewMember{{Session: "a", Title: "Title a"}, {Session: "b", Title: "Title b"}, {Session: "c", Title: "Title c"}}}}}})
			},
			focus:    func(g *Game) int { return top(g).(*walk).member },
			closed:   func(g *Game) bool { _, ok := top(g).(*walk); return !ok },
			verb:     true,
			filterTo: "title c", filtered: func(g *Game) bool { return top(g).(*walk).member == 2 }},
	}
	opened := func(t *testing.T, p panel) *Game {
		t.Helper()
		g := newGame(t)
		p.open(t, g)
		return g
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		defer func() { g.scripted = -1 }()
		require.NoError(t, g.handleKeys())
	}

	for _, p := range panels {
		t.Run("when "+p.name+" is open", func(t *testing.T) {
			for name, key := range map[string]ebiten.Key{"down": ebiten.KeyArrowDown, "j": ebiten.KeyJ} {
				t.Run("and "+name+" is pressed", func(t *testing.T) {
					g := opened(t, p)
					from := p.focus(g)
					press(t, g, key)

					t.Run("it should focus the next row", func(t *testing.T) {
						assert.Equal(t, from+1, p.focus(g))
					})
				})
			}

			for name, key := range map[string]ebiten.Key{"up": ebiten.KeyArrowUp, "k": ebiten.KeyK} {
				t.Run("and down, then "+name+" is pressed", func(t *testing.T) {
					g := opened(t, p)
					from := p.focus(g)
					press(t, g, ebiten.KeyArrowDown)
					press(t, g, key)

					t.Run("it should focus the row it started on", func(t *testing.T) {
						assert.Equal(t, from, p.focus(g))
					})
				})
			}

			t.Run("and tab is pressed", func(t *testing.T) {
				g := opened(t, p)
				from := p.focus(g)
				press(t, g, ebiten.KeyTab)

				t.Run("it should go on down the list", func(t *testing.T) {
					assert.Greater(t, p.focus(g), from)
				})
			})

			t.Run("and esc is pressed", func(t *testing.T) {
				g := opened(t, p)
				press(t, g, ebiten.KeyEscape)

				t.Run("it should go back", func(t *testing.T) {
					assert.True(t, p.closed(g))
				})
			})

			t.Run("and / then "+p.filterTo+" is typed, then enter is pressed", func(t *testing.T) {
				g := opened(t, p)
				press(t, g, ebiten.KeySlash)
				for _, r := range p.filterTo {
					g.scriptedRune = r
					require.NoError(t, g.handleKeys())
					g.scriptedRune = 0
				}
				if !p.verb {
					press(t, g, ebiten.KeyEnter)
				}

				t.Run("it should answer on the one row the filter leaves", func(t *testing.T) {
					assert.True(t, p.filtered(g))
				})
			})

			t.Run("and / then a letter is typed, then esc is pressed", func(t *testing.T) {
				g := opened(t, p)
				press(t, g, ebiten.KeySlash)
				g.scriptedRune = 'o'
				require.NoError(t, g.handleKeys())
				g.scriptedRune = 0
				press(t, g, ebiten.KeyEscape)

				t.Run("it should clear the filter and stay open", func(t *testing.T) {
					assert.False(t, p.closed(g))
				})
			})

			if p.verb {
				t.Run("and 2 is pressed", func(t *testing.T) {
					g := opened(t, p)
					press(t, g, ebiten.KeyDigit2)

					t.Run("it should focus the second row and stay open, since enter is the panel's own", func(t *testing.T) {
						assert.True(t, p.focus(g) == 1 && !p.closed(g))
					})
				})
				return
			}

			t.Run("and enter is pressed", func(t *testing.T) {
				g := opened(t, p)
				press(t, g, ebiten.KeyEnter)

				t.Run("it should choose the focused row", func(t *testing.T) {
					assert.True(t, p.chosen(g))
				})
			})

			if p.docked {
				t.Run("and 2 is pressed", func(t *testing.T) {
					g := opened(t, p)
					from := p.focus(g)
					press(t, g, ebiten.KeyDigit2)

					t.Run("it should leave the digit to the lenses", func(t *testing.T) {
						assert.True(t, p.focus(g) == from && g.scene.Lens() == city.Lenses[1])
					})
				})
				return
			}

			t.Run("and 1 is pressed", func(t *testing.T) {
				g := opened(t, p)
				press(t, g, ebiten.KeyDigit1)

				t.Run("it should choose the first row", func(t *testing.T) {
					assert.True(t, p.chosen(g))
				})
			})
		})
	}

	t.Run("when the groups board is open", func(t *testing.T) {
		opened := func(t *testing.T) *Game {
			t.Helper()
			g := newGame(t)
			g.openBoard()
			press(t, g, ebiten.KeyArrowDown)
			return g
		}
		focused := func(g *Game) state.Group { return g.scene.Groups()[g.board().selected] }

		t.Run("and F2 is pressed", func(t *testing.T) {
			g := opened(t)
			want := focused(g)
			press(t, g, ebiten.KeyF2)

			t.Run("it should rename the focused group", func(t *testing.T) {
				require.NotNil(t, g.picker())
				assert.Equal(t, want.ID, g.picker().renaming)
			})
		})

		t.Run("and Delete is pressed", func(t *testing.T) {
			g := opened(t)
			want := focused(g)
			press(t, g, ebiten.KeyDelete)

			t.Run("it should ask before closing the focused group", func(t *testing.T) {
				require.NotNil(t, g.dialog())
				assert.Equal(t, "Close "+want.Name+"?", g.dialog().title)
			})

			t.Run("and Delete is pressed again", func(t *testing.T) {
				g := opened(t)
				press(t, g, ebiten.KeyDelete)
				press(t, g, ebiten.KeyDelete)

				t.Run("it should close it", func(t *testing.T) {
					assert.True(t, actedOn(g, city.ActionCloseGroup, want.ID))
				})
			})

			t.Run("and esc is pressed", func(t *testing.T) {
				g := opened(t)
				press(t, g, ebiten.KeyDelete)
				press(t, g, ebiten.KeyEscape)

				t.Run("it should close nothing, and leave the board", func(t *testing.T) {
					assert.True(t, !actedOn(g, city.ActionCloseGroup, want.ID) && g.board() != nil)
				})
			})
		})
	})

	t.Run("when the clinic, which renames and removes nothing, is open", func(t *testing.T) {
		for name, key := range map[string]ebiten.Key{"F2": ebiten.KeyF2, "Delete": ebiten.KeyDelete} {
			t.Run("and "+name+" is pressed", func(t *testing.T) {
				g := newGame(t)
				g.openClinic()
				press(t, g, key)

				t.Run("it should leave the clinic as it was", func(t *testing.T) {
					_, open := top(g).(*clinic)
					assert.True(t, open)
				})
			})
		}
	})

	t.Run("when the picker is naming a new group, and / is typed", func(t *testing.T) {
		g := newGame(t)
		g.openPicker("a")
		press(t, g, ebiten.KeyEnter)
		require.True(t, g.picker().naming)
		g.scripted, g.scriptedRune = ebiten.KeySlash, '/'
		require.NoError(t, g.handleKeys())
		g.scripted, g.scriptedRune = -1, 0

		t.Run("it should type the slash into the name, not filter", func(t *testing.T) {
			assert.True(t, g.picker().name.query() == "/" && !g.picker().filter.on)
		})
	})

	t.Run("when diving, and left or right is pressed", func(t *testing.T) {
		g := newGame(t)
		require.True(t, g.scene.FocusGroup("g1"))
		g.scene.StepDive(1)
		step := g.scene.DiveStep()
		press(t, g, ebiten.KeyArrowLeft)

		t.Run("it should leave the station where it was", func(t *testing.T) {
			assert.Equal(t, step, g.scene.DiveStep())
		})
	})
}
