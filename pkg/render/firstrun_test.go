package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestFirstRun(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	groups := []state.Group{
		{ID: "g1", Name: "PROJ-123 · headers", Reason: "mentions PROJ-123",
			Members: []string{"a", "b", "c"}, MemberDirs: []string{"/w/lib", "/w/lib", "/w/api"}},
		{ID: "g2", Name: "LOG-88 · buffer", Reason: "mentions LOG-88",
			Members: []string{"d", "e"}, MemberDirs: []string{"/w/logs", "/w/config"}},
	}
	newGame := func(groups []state.Group, reviewed bool) (*Game, *recordingActor) {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Groups: groups, GroupsReviewed: reviewed,
			Sessions: []state.Session{{ID: "a", CWD: "/w/lib", State: state.Working}}})
		actor := &recordingActor{}
		return &Game{scene: s, actor: actor, scripted: -1, clicked: -1}, actor
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}
	opened := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		g, actor := newGame(groups, false)
		g.maybeFirstRun()
		require.NotNil(t, g.firstRun(), "the first run should be open")
		return g, actor
	}

	cases := []struct {
		scenario string
		groups   []state.Group
		reviewed bool
		open     bool
	}{
		{"the first run has not been reviewed and there are groups", groups, false, true},
		{"the first run has been reviewed", groups, true, false},
		{"there are no groups", nil, false, false},
	}
	for _, c := range cases {
		t.Run("when the city opens and "+c.scenario, func(t *testing.T) {
			g, _ := newGame(c.groups, c.reviewed)
			g.maybeFirstRun()

			t.Run("it should open the first run only when there is something to review", func(t *testing.T) {
				assert.Equal(t, c.open, g.firstRun() != nil)
			})
		})
	}

	t.Run("when the city is only being shot", func(t *testing.T) {
		g, _ := newGame(groups, false)
		g.screenshot = "/tmp/never-written.png"
		g.maybeFirstRun()

		t.Run("it should not open the first run over the frame", func(t *testing.T) {
			assert.Nil(t, g.firstRun())
		})
	})

	t.Run("when the board is open and the first run has not been reviewed", func(t *testing.T) {
		t.Run("and r is pressed", func(t *testing.T) {
			g, _ := newGame(groups, false)
			g.firstRunAsked = true
			g.openBoard()
			press(t, g, ebiten.KeyR)

			t.Run("it should open the first run", func(t *testing.T) {
				assert.NotNil(t, g.firstRun())
			})
		})
	})

	t.Run("when the board is open and the first run has been reviewed", func(t *testing.T) {
		t.Run("and r is pressed", func(t *testing.T) {
			g, _ := newGame(groups, true)
			g.openBoard()
			press(t, g, ebiten.KeyR)

			t.Run("it should not open the first run", func(t *testing.T) {
				assert.Nil(t, g.firstRun())
			})
		})
	})

	t.Run("when the first run is open", func(t *testing.T) {
		t.Run("it should say how many groups it found", func(t *testing.T) {
			g, _ := opened(t)
			assert.Equal(t, "2 groups in the last 30 days", g.firstRun().title())
		})

		fields := []struct {
			field string
			want  string
			get   func(firstRunLine) string
		}{
			{"name", "PROJ-123 · headers", func(l firstRunLine) string { return l.Name }},
			{"sessions", "3 sessions", func(l firstRunLine) string { return l.Sessions }},
			{"directories", "lib, api", func(l firstRunLine) string { return l.Dirs }},
			{"why", "mentions PROJ-123", func(l firstRunLine) string { return l.Why }},
		}
		for _, f := range fields {
			t.Run("a row should show its "+f.field, func(t *testing.T) {
				g, _ := opened(t)
				assert.Equal(t, f.want, f.get(g.firstRun().lines()[0]))
			})
		}

		keys := []struct {
			scenario string
			keys     []ebiten.Key
			check    func(*firstRun) bool
		}{
			{"space drops the chosen row", []ebiten.Key{ebiten.KeySpace}, func(f *firstRun) bool { return !f.review.Rows[0].Keep }},
			{"m on one row and m on another merges them", []ebiten.Key{ebiten.KeyM, ebiten.KeyArrowDown, ebiten.KeyM}, func(f *firstRun) bool { return len(f.review.Rows) == 1 }},
			{"s splits the chosen row by directory", []ebiten.Key{ebiten.KeyS}, func(f *firstRun) bool { return f.review.Rows[0].Name == "PROJ-123 · headers · lib" }},
		}
		for _, k := range keys {
			t.Run("and "+k.scenario, func(t *testing.T) {
				g, _ := opened(t)
				for _, key := range k.keys {
					press(t, g, key)
				}

				t.Run("it should change the rows on the screen", func(t *testing.T) {
					assert.True(t, k.check(g.firstRun()))
				})
			})
		}

		t.Run("and enter confirms", func(t *testing.T) {
			t.Run("it should hand the decisions to the actor", func(t *testing.T) {
				g, actor := opened(t)
				review := g.firstRun().review
				press(t, g, ebiten.KeyEnter)
				assert.Equal(t, []city.Action{{Kind: city.ActionReviewGroups, Review: &review}}, actor.done)
			})

			t.Run("it should close", func(t *testing.T) {
				g, _ := opened(t)
				press(t, g, ebiten.KeyEnter)
				assert.Nil(t, g.firstRun())
			})
		})

		for _, key := range []ebiten.Key{ebiten.KeyEscape, ebiten.KeyA} {
			t.Run("and "+key.String()+" asks later", func(t *testing.T) {
				t.Run("it should close", func(t *testing.T) {
					g, _ := opened(t)
					press(t, g, key)
					assert.Nil(t, g.firstRun())
				})

				t.Run("it should write nothing", func(t *testing.T) {
					g, actor := opened(t)
					press(t, g, key)
					assert.Empty(t, actor.done)
				})

				t.Run("it should not open again in this run", func(t *testing.T) {
					g, _ := opened(t)
					press(t, g, key)
					g.maybeFirstRun()
					assert.Nil(t, g.firstRun())
				})
			})
		}
	})
}
