package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 333: the tidy-up walk, one card per decision, strongest group
// first, then the quiet sessions, pre-answered pack them up.
func TestTidyWalk(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	session := func(id, gid string, st state.State, quiet time.Duration) state.Session {
		return state.Session{ID: id, Title: "T " + id, CWD: "/w/" + id, State: st, Group: gid,
			StartedAt: now.Add(-quiet - time.Hour), LastActivity: now.Add(-quiet)}
	}
	likely := func(s state.Session) state.Session {
		s.GroupLikely = true
		return s
	}
	resting := func(id, gid string) state.Session { return session(id, gid, state.Resting, 2*day) }
	sessions := func() []state.Session {
		return []state.Session{
			session("a", "live", state.Working, 0), resting("b", "live"), resting("c", "live"),
			resting("d", "four"), resting("e", "four"), resting("f", "four"), resting("h", "four"),
			resting("i", "sure3"), resting("j", "sure3"), resting("k", "sure3"),
			resting("l", "likely3"), likely(resting("m", "likely3")), resting("n", "likely3"),
			resting("p", "weak"), likely(resting("q", "weak")),
			resting("r", "pair"), resting("s", "pair"),
			session("old1", "", state.Resting, 10*day), session("old2", "", state.Resting, 12*day),
			session("mid", "", state.Resting, 4*day), session("fresh", "", state.Resting, day),
		}
	}
	groupOf := func(id string, all []state.Session) state.Group {
		g := state.Group{ID: id, Name: "group " + id, Journal: state.Journal{Active: now}}
		for _, s := range all {
			if s.Group == id {
				g.Members = append(g.Members, s.ID)
				g.MemberDirs = append(g.MemberDirs, s.CWD)
				g.MemberTitles = append(g.MemberTitles, s.Title)
			}
		}
		return g
	}
	type swept struct {
		ids      []string
		quietFor time.Duration
	}
	newGame := func(t *testing.T, away time.Time) (*Game, *recordingActor, *[]swept) {
		t.Helper()
		all := sessions()
		var groups []state.Group
		for _, id := range []string{"pair", "weak", "likely3", "sure3", "four", "live"} {
			groups = append(groups, groupOf(id, all))
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(1280, 800)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: all, Groups: groups})
		actor := &recordingActor{}
		calls := &[]swept{}
		g := &Game{scene: s, actor: actor, theme: ui.NewTheme(1), scripted: -1, clicked: -1, quietFor: 7 * day,
			journalHours: 8, awaySince: away, dispatch: func(run func()) { run() }}
		g.sweep = func(ids []string, quietFor time.Duration) (int, error) {
			*calls = append(*calls, swept{ids, quietFor})
			return len(ids), nil
		}
		return g, actor, calls
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
		g.acting.Wait()
	}
	walking := func(t *testing.T) (*Game, *recordingActor, *[]swept) {
		t.Helper()
		g, actor, calls := newGame(t, time.Time{})
		require.True(t, g.openWalk(), "the walk should open")
		require.NotNil(t, g.walk())
		return g, actor, calls
	}
	// through is the walk past the Start card, onto the first group's.
	through := func(t *testing.T) (*Game, *recordingActor, *[]swept) {
		t.Helper()
		g, actor, calls := walking(t)
		press(t, g, ebiten.KeyEnter)
		return g, actor, calls
	}
	reviewed := func(t *testing.T, actor *recordingActor) group.Review {
		t.Helper()
		for _, a := range actor.done {
			if a.Kind == city.ActionReviewGroups {
				require.NotNil(t, a.Review)
				return *a.Review
			}
		}
		t.Fatalf("no review written: %v", actor.done)
		return group.Review{}
	}
	rowNamed := func(r group.Review, id string) (group.ReviewRow, bool) {
		for _, row := range r.Rows {
			if row.ID == id {
				return row, true
			}
		}
		return group.ReviewRow{}, false
	}

	t.Run("when the walk opens", func(t *testing.T) {
		g, _, _ := walking(t)

		t.Run("its cards should be Start, the groups strongest first, then the quiet", func(t *testing.T) {
			assert.Equal(t, []string{"start", "live", "four", "sure3", "likely3", "pair", "quiet"}, g.walk().cardKeys())
		})

		t.Run("it should leave the weak pair off, as a suggestion only", func(t *testing.T) {
			_, ok := rowNamed(g.walk().review, "weak")
			assert.False(t, ok)
		})

		t.Run("its title should count the cards and estimate the time", func(t *testing.T) {
			assert.Equal(t, "Tidy up · 1 of 7 · about 2 minutes", g.walk().title())
		})

		t.Run("its first card should offer Start clean, Walk through it and Later", func(t *testing.T) {
			assert.Equal(t, []string{"c  Start clean", "enter  Walk through it", "l  Later"}, g.walk().buttonLabels())
		})
	})

	t.Run("when the walk moves onto a group's card", func(t *testing.T) {
		g, _, _ := through(t)

		t.Run("it should focus the map on that group", func(t *testing.T) {
			assert.Equal(t, "live", g.scene.FocusedGroup())
		})

		t.Run("it should list the group's members", func(t *testing.T) {
			assert.Equal(t, []string{"T a", "T b", "T c"}, g.walk().memberTitles())
		})

		t.Run("it should offer a button for every key", func(t *testing.T) {
			assert.Equal(t, []string{"enter  keep", "r  rename", "x  drop", "s  split", "-  take one out", "n  next", "p  back"},
				g.walk().buttonLabels())
		})

		t.Run("and a button is clicked", func(t *testing.T) {
			g, _, _ := through(t)
			w := g.walk()
			w.layout(g.theme, 1280, 800, func(s string, _ ui.Size) (float64, float64) { return 7 * float64(len([]rune(s))), 14 })
			require.True(t, w.click(g, w.buttons[0].rect.Center()))

			t.Run("it should do what its key does", func(t *testing.T) {
				assert.Equal(t, "four", g.walk().cardKeys()[g.walk().at])
			})
		})
	})

	for _, c := range []struct {
		name string
		keys []ebiten.Key
		want func(group.ReviewRow) bool
	}{
		{"enter keeps it", []ebiten.Key{ebiten.KeyEnter}, func(r group.ReviewRow) bool { return r.Keep && r.Decided }},
		{"x drops it", []ebiten.Key{ebiten.KeyX}, func(r group.ReviewRow) bool { return !r.Keep && r.Decided }},
		{"j then - takes the second member out", []ebiten.Key{ebiten.KeyJ, ebiten.KeyMinus},
			func(r group.ReviewRow) bool { return len(r.Members) == 2 && r.Members[1].Session == "c" }},
	} {
		t.Run("when on a group's card "+c.name, func(t *testing.T) {
			g, _, _ := through(t)
			for _, k := range c.keys {
				press(t, g, k)
			}
			row, _ := rowNamed(g.walk().review, "live")

			t.Run("it should record that on the group's row", func(t *testing.T) {
				assert.True(t, c.want(row), "%+v", row)
			})
		})
	}

	t.Run("when on a group's card s splits it", func(t *testing.T) {
		g, _, _ := through(t)
		press(t, g, ebiten.KeyS)

		t.Run("it should split the row by directory", func(t *testing.T) {
			_, ok := rowNamed(g.walk().review, "live")
			assert.False(t, ok)
		})

		t.Run("it should move to the next card", func(t *testing.T) {
			assert.Equal(t, "four", g.walk().cardKeys()[g.walk().at])
		})
	})

	t.Run("when on a group's card r renames it", func(t *testing.T) {
		g, actor, _ := through(t)
		press(t, g, ebiten.KeyR)
		require.NotNil(t, g.picker(), "the rename panel should open over the walk")
		for range len([]rune(g.picker().name.query())) {
			press(t, g, ebiten.KeyBackspace)
		}
		for _, r := range "Limiter" {
			g.scriptedRune = r
			require.NoError(t, g.handleKeys())
			g.scriptedRune = 0
		}
		press(t, g, ebiten.KeyEnter)
		row, _ := rowNamed(g.walk().review, "live")

		t.Run("it should rename the row, to be written when the walk is left", func(t *testing.T) {
			assert.Equal(t, [2]any{"Limiter", true}, [2]any{row.Name, row.Renamed})
		})

		t.Run("it should write nothing yet", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})
	})

	t.Run("when n moves on and p comes back", func(t *testing.T) {
		g, _, _ := through(t)
		press(t, g, ebiten.KeyN)
		press(t, g, ebiten.KeyP)

		t.Run("it should be on the first group's card again, undecided", func(t *testing.T) {
			row, _ := rowNamed(g.walk().review, "live")
			assert.Equal(t, [2]any{"live", false}, [2]any{g.walk().cardKeys()[g.walk().at], row.Decided})
		})
	})

	t.Run("when the walk reaches the quiet card", func(t *testing.T) {
		quiet := func(t *testing.T) (*Game, *recordingActor, *[]swept) {
			t.Helper()
			g, actor, calls := through(t)
			for range 5 {
				press(t, g, ebiten.KeyN)
			}
			require.Equal(t, "quiet", g.walk().cardKeys()[g.walk().at])
			return g, actor, calls
		}
		g, _, _ := quiet(t)

		t.Run("it should count the quiet sessions and say it will pack them up", func(t *testing.T) {
			assert.Equal(t, "2 sessions in 2 directories have been quiet for more than 7 days and belong to no group — pack them up?", g.walk().quietLine())
		})

		t.Run("and v is pressed, it should show their districts", func(t *testing.T) {
			g, _, _ := quiet(t)
			press(t, g, ebiten.KeyV)
			assert.Equal(t, []string{"old1", "old2"}, g.walk().quietDistricts())
		})

		t.Run("and [ turns the threshold down a day", func(t *testing.T) {
			g, _, _ := quiet(t)
			for range 3 {
				press(t, g, ebiten.KeyBracketLeft)
			}

			t.Run("it should count what is quiet that long", func(t *testing.T) {
				assert.Equal(t, 3, len(g.walk().quietIDs()))
			})
		})

		t.Run("and enter is pressed, as answered", func(t *testing.T) {
			g, actor, calls := quiet(t)
			press(t, g, ebiten.KeyEnter)

			t.Run("it should pack the quiet sessions up", func(t *testing.T) {
				assert.Equal(t, []swept{{ids: []string{"old1", "old2"}, quietFor: 7 * day}}, *calls)
			})

			t.Run("it should confirm the groups, every row as it stands", func(t *testing.T) {
				assert.Len(t, reviewed(t, actor).Rows, 5)
			})

			t.Run("it should close the walk and open the journal", func(t *testing.T) {
				_, journalOpen := g.overlays.top().(*journal)
				assert.Equal(t, [2]bool{true, true}, [2]bool{g.walk() == nil, journalOpen})
			})
		})

		t.Run("and x leaves them", func(t *testing.T) {
			g, _, calls := quiet(t)
			press(t, g, ebiten.KeyX)

			t.Run("it should pack nothing", func(t *testing.T) {
				assert.Empty(t, *calls)
			})
		})
	})

	t.Run("when esc is pressed before anything is decided", func(t *testing.T) {
		g, actor, calls := through(t)
		press(t, g, ebiten.KeyEscape)

		t.Run("it should ask later: write nothing", func(t *testing.T) {
			assert.Equal(t, [2]int{0, 0}, [2]int{len(actor.done), len(*calls)})
		})

		t.Run("it should close the walk", func(t *testing.T) {
			assert.Nil(t, g.walk())
		})
	})

	t.Run("when Later is chosen on the Start card", func(t *testing.T) {
		g, actor, _ := walking(t)
		press(t, g, ebiten.KeyL)

		t.Run("it should write nothing and close the walk", func(t *testing.T) {
			assert.Equal(t, [2]any{0, true}, [2]any{len(actor.done), g.walk() == nil})
		})
	})

	t.Run("when esc is pressed after a decision", func(t *testing.T) {
		g, actor, calls := through(t)
		press(t, g, ebiten.KeyX)
		press(t, g, ebiten.KeyEscape)
		r := reviewed(t, actor)

		t.Run("it should write only what was decided, for now", func(t *testing.T) {
			assert.Equal(t, []string{"live"}, rowIDs(r))
		})

		t.Run("it should pack nothing, the quiet card not reached", func(t *testing.T) {
			assert.Empty(t, *calls)
		})
	})

	t.Run("when Start clean is chosen on the first run", func(t *testing.T) {
		g, actor, calls := walking(t)
		press(t, g, ebiten.KeyC)

		t.Run("it should pack up the quiet sessions", func(t *testing.T) {
			assert.Equal(t, []swept{{ids: []string{"old1", "old2"}, quietFor: 7 * day}}, *calls)
		})

		t.Run("it should confirm every group the walk would ask about", func(t *testing.T) {
			assert.Len(t, reviewed(t, actor).Rows, 5)
		})

		t.Run("it should close the walk", func(t *testing.T) {
			assert.Nil(t, g.walk())
		})
	})

	t.Run("when Start clean is chosen on a return after three days away", func(t *testing.T) {
		g, _, calls := newGame(t, now.Add(-3*day))
		require.True(t, g.openWalk())
		press(t, g, ebiten.KeyC)

		t.Run("it should pack up what in no group was resting before you left", func(t *testing.T) {
			require.Len(t, *calls, 1)
			assert.ElementsMatch(t, []string{"old1", "old2", "mid"}, (*calls)[0].ids)
		})
	})

	t.Run("when t is pressed on City Hall's board", func(t *testing.T) {
		g, _, _ := newGame(t, time.Time{})
		g.driven = true
		press(t, g, ebiten.KeyG)
		require.NotNil(t, g.board())
		press(t, g, ebiten.KeyT)

		t.Run("it should open the walk in the board's place, even in a driven run", func(t *testing.T) {
			assert.Equal(t, [2]bool{true, false}, [2]bool{g.walk() != nil, g.board() != nil})
		})
	})

	t.Run("when the city opens unreviewed", func(t *testing.T) {
		for _, c := range []struct {
			name string
			set  func(*Game)
			open bool
		}{
			{"for someone looking", func(*Game) {}, true},
			{"for a screenshot", func(g *Game) { g.screenshot = "x.png" }, false},
			{"for a recording", func(g *Game) { g.record = "x.gif" }, false},
			{"for a driven run", func(g *Game) { g.driven = true }, false},
		} {
			t.Run(c.name, func(t *testing.T) {
				g, _, _ := newGame(t, time.Time{})
				c.set(g)
				g.maybeFirstRun()

				t.Run("it should open the walk only for someone looking", func(t *testing.T) {
					assert.Equal(t, c.open, g.walk() != nil)
				})
			})
		}
	})
}

func rowIDs(r group.Review) []string {
	var ids []string
	for _, row := range r.Rows {
		ids = append(ids, row.ID)
	}
	return ids
}
