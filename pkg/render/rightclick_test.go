package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350, slice 3: a right-click opens the action menu at the pointer,
// on a building, a crate, a district or a row of the groups board, and
// a right drag still pans.
func TestRightClick(t *testing.T) {
	right := ebiten.MouseButtonRight
	click := func(g *Game, at city.Point) {
		g.scene.PointerMove(at)
		g.buttonPan(at, pressOf(right))
		g.rightButton(at, pressOf(right))
		g.buttonPan(at, releaseOf(right))
		g.rightButton(at, releaseOf(right))
	}
	rows := func(g *Game) [][]city.ActionKind { return g.menu().kinds }
	press := func(g *Game, key ebiten.Key) {
		g.scripted = key
		defer func() { g.scripted = -1 }()
		require.NoError(t, g.handleKeys())
	}

	t.Run("when a building is right-clicked", func(t *testing.T) {
		r := newButtonPanRig(t)
		r.g.scene.Deselect()
		at := r.g.scene.Camera().WorldToScreen(r.selected.Rect.Center())
		click(r.g, at)

		t.Run("it should select it", func(t *testing.T) {
			assert.Equal(t, r.selected, r.g.scene.Selected())
		})

		t.Run("it should open the menu at the pointer", func(t *testing.T) {
			require.NotNil(t, r.g.menu())
			assert.Equal(t, at, r.g.menu().at)
		})

		t.Run("it should offer the building's verbs, every column", func(t *testing.T) {
			v := r.g.scene.Verbs()
			assert.Equal(t, v.Columns(), rows(r.g))
		})

		t.Run("it should arm the first column with rows", func(t *testing.T) {
			assert.Equal(t, []any{true, ui.MenuFocus{Column: 0, Row: 0}}, []any{r.g.menu().armed, r.g.menu().focus})
		})
	})

	t.Run("when a crate is right-clicked", func(t *testing.T) {
		now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetInstant(true)
		s.Resize(1600, 1200)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{
			{ID: "a", CWD: "/w/lib", State: state.Working},
			{ID: "p", CWD: "/w/lib", State: state.Resting, Packed: true},
		}})
		g := &Game{scene: s, actor: &recordingActor{}, scripted: -1, clicked: -1}
		var crate *city.Building
		for _, b := range s.City().Buildings() {
			if b.Session.ID == "p" {
				crate = b
			}
		}
		require.NotNil(t, crate)
		click(g, s.Camera().WorldToScreen(crate.Rect.Center()))

		t.Run("the Session column should be resume, unpack, demolish", func(t *testing.T) {
			require.NotNil(t, g.menu())
			assert.Equal(t, []city.ActionKind{city.ActionResume, city.ActionUnpack, city.ActionDemolish}, rows(g)[0])
		})
	})

	t.Run("when the right button is dragged past the slop", func(t *testing.T) {
		r := newButtonPanRig(t)
		from := r.selectedBuilding(t)
		before := r.g.scene.Camera().Offset
		to := from.Add(city.Point{X: 40, Y: 30})
		r.g.buttonPan(from, pressOf(right))
		r.g.rightButton(from, pressOf(right))
		r.g.scene.PointerMove(to)
		r.g.buttonPan(to, holdOf(right))
		r.g.rightButton(to, holdOf(right))
		r.g.buttonPan(to, releaseOf(right))
		r.g.rightButton(to, releaseOf(right))

		t.Run("it should pan", func(t *testing.T) {
			assert.NotEqual(t, before, r.g.scene.Camera().Offset)
		})

		t.Run("it should open no menu", func(t *testing.T) {
			assert.Nil(t, r.g.menu())
		})
	})

	t.Run("when a district's ground is right-clicked", func(t *testing.T) {
		r := newButtonPanRig(t)
		at := r.districtFloor(t)
		d := r.g.scene.Hover().District
		click(r.g, at)

		t.Run("it should open the menu with the Project column alone", func(t *testing.T) {
			require.NotNil(t, r.g.menu())
			assert.Equal(t, [][]city.ActionKind{nil, r.g.scene.DistrictVerbs(d), nil}, rows(r.g))
		})

		t.Run("it should leave the selection as it was", func(t *testing.T) {
			assert.Equal(t, r.selected, r.g.scene.Selected())
		})

		t.Run("and copy path's letter is pressed", func(t *testing.T) {
			press(r.g, ebiten.KeyC)
			r.g.settle()

			t.Run("it should copy the district's root", func(t *testing.T) {
				assert.Contains(t, r.actor.done, city.Action{Kind: city.ActionCopyPath, Dir: d.Root})
			})
		})
	})

	t.Run("when open ground is right-clicked", func(t *testing.T) {
		r := newButtonPanRig(t)
		click(r.g, r.openGround(t))

		t.Run("it should open no menu", func(t *testing.T) {
			assert.Nil(t, r.g.menu())
		})
	})

	t.Run("when the city is covered by a panel, and the map is right-clicked", func(t *testing.T) {
		r := newButtonPanRig(t)
		at := r.selectedBuilding(t)
		r.g.settingsOpen = true
		click(r.g, at)

		t.Run("it should open no menu", func(t *testing.T) {
			assert.Nil(t, r.g.menu())
		})
	})

	t.Run("when the pointer is on the card, and the right button clicks", func(t *testing.T) {
		r := newButtonPanRig(t)
		at := r.selectedBuilding(t)
		r.g.pinned = ui.Card{Rect: city.RectAt(at.X-50, at.Y-50, 100, 100)}
		click(r.g, at)

		t.Run("it should open no menu", func(t *testing.T) {
			assert.Nil(t, r.g.menu())
		})
	})

	t.Run("when a groups-board row is right-clicked", func(t *testing.T) {
		now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
		open := func(t *testing.T) (*Game, *recordingActor, city.Point) {
			t.Helper()
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return now }, time.UTC)
			s.SetInstant(true)
			s.Resize(1600, 1200)
			s.SetSnapshot(state.Snapshot{At: now,
				Sessions: []state.Session{
					{ID: "a", CWD: "/w/lib", State: state.Working, Group: "g1", LastActivity: now},
					{ID: "b", CWD: "/w/api", State: state.Working, Group: "g1", LastActivity: now},
					{ID: "c", CWD: "/w/web", State: state.Working, Group: "g2", LastActivity: now},
					{ID: "d", CWD: "/w/doc", State: state.Working, Group: "g2", LastActivity: now},
				},
				Groups: []state.Group{
					{ID: "g1", Name: "Lib and api", Members: []string{"a", "b"}},
					{ID: "g2", Name: "Web and docs", Members: []string{"c", "d"}},
				}})
			actor := &recordingActor{}
			g := &Game{scene: s, actor: actor, scripted: -1, clicked: -1}
			g.openBoard()
			b := g.board()
			b.rowRects = []city.Rect{city.RectAt(100, 100, 400, 60), city.RectAt(100, 160, 400, 60)}
			at := b.rowRects[1].Center()
			g.buttonPan(at, pressOf(right))
			g.rightButton(at, pressOf(right))
			g.buttonPan(at, releaseOf(right))
			g.rightButton(at, releaseOf(right))
			return g, actor, at
		}

		t.Run("it should choose that row", func(t *testing.T) {
			g, _, _ := open(t)
			b, _ := g.overlays[0].(*board)
			assert.Equal(t, 1, b.selected)
		})

		t.Run("it should open the Group column for that group", func(t *testing.T) {
			g, _, _ := open(t)
			require.NotNil(t, g.menu())
			assert.Equal(t, [][]city.ActionKind{nil, nil, {city.ActionRenameGroup, city.ActionCloseGroup, city.ActionCopyStandup, city.ActionFocusLine}}, rows(g))
		})

		t.Run("and close group is chosen", func(t *testing.T) {
			g, actor, _ := open(t)
			press(g, ebiten.KeyC)

			t.Run("it should ask first, closing nothing yet", func(t *testing.T) {
				d := g.dialog()
				require.NotNil(t, d)
				assert.True(t, d.title == "Close Web and docs?" && len(actor.done) == 0, "%q %v", d.title, actor.done)
			})

			t.Run("and Delete confirms it", func(t *testing.T) {
				press(g, ebiten.KeyDelete)

				t.Run("it should close that group", func(t *testing.T) {
					require.NotEmpty(t, actor.done)
					last := actor.done[len(actor.done)-1]
					assert.Equal(t, []any{city.ActionCloseGroup, "g2"}, []any{last.Kind, last.Group})
				})
			})
		})

		t.Run("and focus the line is chosen", func(t *testing.T) {
			g, _, _ := open(t)
			press(g, ebiten.KeyF)

			t.Run("it should focus that group", func(t *testing.T) {
				assert.Equal(t, "g2", g.scene.FocusedGroup())
			})

			t.Run("it should close the board", func(t *testing.T) {
				assert.Nil(t, g.board())
			})
		})
	})
}
