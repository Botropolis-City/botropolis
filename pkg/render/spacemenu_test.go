package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350, slice 4: space opens the action menu from the keyboard, on
// the selection's Session column with nothing chosen yet, so a column's
// letter and then a row's runs a verb; with nothing selected it opens
// the district under the pointer's.
func TestSpaceMenu(t *testing.T) {
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		defer func() { g.scripted = -1 }()
		require.NoError(t, g.handleKeys())
	}
	spaced := func(t *testing.T) *buttonPanRig {
		t.Helper()
		r := newButtonPanRig(t)
		press(t, r.g, ebiten.KeySpace)
		return r
	}

	t.Run("when a session is selected and space is pressed", func(t *testing.T) {
		t.Run("it should open the menu", func(t *testing.T) {
			r := spaced(t)
			assert.NotNil(t, r.g.menu())
		})

		t.Run("it should open it on the Session column, with nothing chosen", func(t *testing.T) {
			r := spaced(t)
			require.NotNil(t, r.g.menu())
			assert.Equal(t, []any{false, ui.MenuFocus{Column: 0, Row: -1}}, []any{r.g.menu().armed, r.g.menu().focus})
		})

		t.Run("it should offer the selection's verbs", func(t *testing.T) {
			r := spaced(t)
			require.NotNil(t, r.g.menu())
			assert.Equal(t, r.g.scene.Verbs().Columns(), r.g.menu().kinds)
		})

		for _, c := range []struct {
			name       string
			col, row   ebiten.Key
			want       city.ActionKind
			wantPicker bool
		}{
			{name: "s then p", col: ebiten.KeyS, row: ebiten.KeyP, want: city.ActionPack},
			{name: "p then n", col: ebiten.KeyP, row: ebiten.KeyN, want: city.ActionNew},
			{name: "g then m", col: ebiten.KeyG, row: ebiten.KeyM, wantPicker: true},
		} {
			t.Run("and "+c.name+" is pressed", func(t *testing.T) {
				r := spaced(t)
				press(t, r.g, c.col)
				press(t, r.g, c.row)
				r.g.settle()

				if c.wantPicker {
					t.Run("it should open the group picker", func(t *testing.T) {
						assert.NotNil(t, r.g.picker())
					})
					return
				}
				t.Run("it should run "+string(c.want), func(t *testing.T) {
					var kinds []city.ActionKind
					for _, a := range r.actor.done {
						kinds = append(kinds, a.Kind)
					}
					assert.Contains(t, kinds, c.want)
				})
			})
		}
	})

	t.Run("when nothing is selected, a district is under the pointer, and space is pressed", func(t *testing.T) {
		r := newButtonPanRig(t)
		r.g.scene.Deselect()
		at := r.districtFloor(t)
		r.g.hover, r.g.hoverSet = at, true
		d := r.g.scene.Hover().District
		press(t, r.g, ebiten.KeySpace)

		t.Run("it should open the Project column for that district", func(t *testing.T) {
			require.NotNil(t, r.g.menu())
			assert.Equal(t, [][]city.ActionKind{nil, r.g.scene.DistrictVerbs(d), nil}, r.g.menu().kinds)
		})
	})

	t.Run("when nothing is selected or under the pointer, and space is pressed", func(t *testing.T) {
		r := newButtonPanRig(t)
		r.g.scene.Deselect()
		r.openGround(t)
		press(t, r.g, ebiten.KeySpace)

		t.Run("it should open nothing", func(t *testing.T) {
			assert.Nil(t, r.g.menu())
		})
	})

	t.Run("when the settings are open and space is pressed", func(t *testing.T) {
		r := newButtonPanRig(t)
		r.g.settingsOpen = true
		press(t, r.g, ebiten.KeySpace)

		t.Run("it should leave the settings their own space", func(t *testing.T) {
			assert.True(t, r.g.settingsOpen && r.g.menu() == nil)
		})
	})
}

// Item 350, slice 4: the keys that became menu rows do nothing at the
// map, and the journal is j.
func TestRetiredKeys(t *testing.T) {
	press := func(t *testing.T, g *Game, key ebiten.Key, shift bool) {
		t.Helper()
		g.scripted, g.scriptedShift = key, shift
		defer func() { g.scripted, g.scriptedShift = -1, false }()
		require.NoError(t, g.handleKeys())
	}

	t.Run("when shift+p is pressed with a session selected", func(t *testing.T) {
		r := newButtonPanRig(t)
		press(t, r.g, ebiten.KeyP, true)

		t.Run("it should pack nothing", func(t *testing.T) {
			assert.Empty(t, r.actor.done)
		})

		t.Run("it should save no screenshot", func(t *testing.T) {
			assert.Empty(t, r.g.snap)
		})
	})

	t.Run("when p is pressed alone", func(t *testing.T) {
		r := newButtonPanRig(t)
		press(t, r.g, ebiten.KeyP, false)

		t.Run("it should still save a screenshot", func(t *testing.T) {
			assert.NotEmpty(t, r.g.snap)
		})
	})
}
