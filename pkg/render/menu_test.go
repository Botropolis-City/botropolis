package render

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350: the action menu is a panel over the city that answers its
// column letters, its row letters, the arrows, enter and escape, and
// closes on a click outside.
func TestMenuPanel(t *testing.T) {
	const width, height = 1280.0, 800.0
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	live := state.Session{ID: "live", Title: "Fix the queue", CWD: "/src/kiln", State: state.NeedsYou, Kind: claude.KindBackground, LastActivity: at, StartedAt: at.Add(-time.Hour)}
	kinds := []city.ActionKind{city.ActionAttach, city.ActionStop, city.ActionRead, city.ActionNew, city.ActionCopyPath, city.ActionMoveToGroup}

	type rig struct {
		g      *Game
		chosen []city.ActionKind
	}
	newRig := func(t *testing.T) *rig {
		t.Helper()
		r := &rig{}
		scene := city.NewScene(city.NewLayout())
		scene.Resize(width, height)
		scene.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{live}})
		require.True(t, scene.Select("live"))
		r.g = &Game{scene: scene, actor: &recorder{}, scripted: -1, clicked: -1}
		return r
	}
	open := func(t *testing.T, column int) *rig {
		t.Helper()
		r := newRig(t)
		r.g.openMenu(kinds, city.Point{X: 300, Y: 300}, column)
		r.g.menu().choose = func(kind city.ActionKind) { r.chosen = append(r.chosen, kind) }
		return r
	}
	press := func(g *Game, key ebiten.Key) {
		g.scripted = key
		defer func() { g.scripted = -1 }()
		require.NoError(t, g.handleKeys())
	}
	menuOpen := func(g *Game) bool { return g.menu() != nil }
	laid := func(g *Game) ui.Menu { return g.menu().layout(g, width, height) }

	t.Run("when the menu opens on no column", func(t *testing.T) {
		t.Run("it should be the top overlay", func(t *testing.T) {
			r := open(t, -1)
			assert.True(t, menuOpen(r.g))
		})

		t.Run("it should focus the first column and no row", func(t *testing.T) {
			r := open(t, -1)
			assert.Equal(t, ui.MenuFocus{Column: 0, Row: -1}, r.g.menu().focus)
		})

		t.Run("and a column's letter is pressed", func(t *testing.T) {
			r := open(t, -1)
			press(r.g, ebiten.KeyP)

			t.Run("it should focus that column's first row", func(t *testing.T) {
				assert.Equal(t, ui.MenuFocus{Column: 1, Row: 0}, r.g.menu().focus)
			})
		})

		for name, key := range map[string]ebiten.Key{"down": ebiten.KeyArrowDown, "j": ebiten.KeyJ} {
			t.Run("and "+name+" is pressed", func(t *testing.T) {
				r := open(t, -1)
				press(r.g, key)

				t.Run("it should focus the first column's first row", func(t *testing.T) {
					assert.Equal(t, ui.MenuFocus{Column: 0, Row: 0}, r.g.menu().focus)
				})

				t.Run("and again past the last row", func(t *testing.T) {
					for range 5 {
						press(r.g, key)
					}

					t.Run("it should stay on the last row", func(t *testing.T) {
						assert.Equal(t, ui.MenuFocus{Column: 0, Row: 2}, r.g.menu().focus)
					})
				})
			})
		}

		for name, key := range map[string]ebiten.Key{"up": ebiten.KeyArrowUp, "k": ebiten.KeyK} {
			t.Run("and "+name+" is pressed at the top", func(t *testing.T) {
				r := open(t, -1)
				press(r.g, ebiten.KeyArrowDown)
				press(r.g, key)

				t.Run("it should stay on the first row", func(t *testing.T) {
					assert.Equal(t, ui.MenuFocus{Column: 0, Row: 0}, r.g.menu().focus)
				})
			})
		}

		t.Run("and enter is pressed with a row focused", func(t *testing.T) {
			r := open(t, -1)
			press(r.g, ebiten.KeyArrowDown)
			press(r.g, ebiten.KeyArrowDown)
			press(r.g, ebiten.KeyEnter)

			t.Run("it should run that row's action", func(t *testing.T) {
				assert.Equal(t, []city.ActionKind{city.ActionStop}, r.chosen)
			})

			t.Run("it should close", func(t *testing.T) {
				assert.False(t, menuOpen(r.g))
			})
		})

		t.Run("and enter is pressed with no row focused", func(t *testing.T) {
			r := open(t, -1)
			press(r.g, ebiten.KeyEnter)

			t.Run("it should run nothing and stay open", func(t *testing.T) {
				assert.True(t, len(r.chosen) == 0 && menuOpen(r.g))
			})
		})

		t.Run("and a row's letter is pressed with no column chosen", func(t *testing.T) {
			r := open(t, -1)
			press(r.g, ebiten.KeyA)

			t.Run("it should run nothing", func(t *testing.T) {
				assert.Empty(t, r.chosen)
			})
		})

		t.Run("and escape is pressed", func(t *testing.T) {
			r := open(t, -1)
			press(r.g, ebiten.KeyEscape)

			t.Run("it should close", func(t *testing.T) {
				assert.False(t, menuOpen(r.g))
			})

			t.Run("it should leave the selection selected", func(t *testing.T) {
				assert.NotNil(t, r.g.scene.Selected())
			})
		})
	})

	t.Run("when the menu opens on a column", func(t *testing.T) {
		t.Run("it should focus that column's first row", func(t *testing.T) {
			r := open(t, 1)
			assert.Equal(t, ui.MenuFocus{Column: 1, Row: 0}, r.g.menu().focus)
		})

		t.Run("and a row's letter is pressed", func(t *testing.T) {
			r := open(t, 1)
			press(r.g, ebiten.KeyC)

			t.Run("it should run that row's action", func(t *testing.T) {
				assert.Equal(t, []city.ActionKind{city.ActionCopyPath}, r.chosen)
			})

			t.Run("it should close", func(t *testing.T) {
				assert.False(t, menuOpen(r.g))
			})
		})

		for name, key := range map[string]ebiten.Key{"right": ebiten.KeyArrowRight, "tab": ebiten.KeyTab} {
			t.Run("and "+name+" is pressed", func(t *testing.T) {
				r := open(t, 1)
				press(r.g, key)

				t.Run("it should focus the next column's first row", func(t *testing.T) {
					assert.Equal(t, ui.MenuFocus{Column: 2, Row: 0}, r.g.menu().focus)
				})
			})
		}

		t.Run("and left is pressed", func(t *testing.T) {
			r := open(t, 1)
			press(r.g, ebiten.KeyArrowLeft)

			t.Run("it should focus the previous column's first row", func(t *testing.T) {
				assert.Equal(t, ui.MenuFocus{Column: 0, Row: 0}, r.g.menu().focus)
			})
		})
	})

	t.Run("when a row is clicked", func(t *testing.T) {
		r := open(t, -1)
		row := laid(r.g).Rows[0][1].Rect.Center()
		taken := r.g.clickOverlay(row)

		t.Run("it should run its action", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionStop}, r.chosen)
		})

		t.Run("it should close, having taken the click", func(t *testing.T) {
			assert.True(t, taken && !menuOpen(r.g))
		})
	})

	t.Run("when a chosen row opens a panel of its own", func(t *testing.T) {
		r := newRig(t)
		r.g.openMenu(kinds, city.Point{X: 300, Y: 300}, 2)
		press(r.g, ebiten.KeyM)

		t.Run("it should leave that panel on top", func(t *testing.T) {
			assert.NotNil(t, r.g.picker())
		})

		t.Run("it should take the menu away", func(t *testing.T) {
			for _, o := range r.g.overlays {
				_, isMenu := o.(*menuPanel)
				assert.False(t, isMenu)
			}
		})
	})

	t.Run("when a click lands outside", func(t *testing.T) {
		r := open(t, -1)
		taken := r.g.clickOverlay(city.Point{X: 10, Y: 10})

		t.Run("it should close", func(t *testing.T) {
			assert.False(t, menuOpen(r.g))
		})

		t.Run("it should take the click, so it goes no further", func(t *testing.T) {
			assert.True(t, taken && r.g.scene.Selected() != nil)
		})
	})

	t.Run("when a click lands on the menu beside its rows", func(t *testing.T) {
		r := open(t, -1)
		r.g.clickOverlay(laid(r.g).Rect.Min.Add(city.Point{X: 2, Y: 2}))

		t.Run("it should stay open", func(t *testing.T) {
			assert.True(t, menuOpen(r.g))
		})
	})

	t.Run("when the menu is open", func(t *testing.T) {
		r := open(t, -1)

		t.Run("it should cover the city", func(t *testing.T) {
			assert.True(t, r.g.cityCovered())
		})

		t.Run("it should lay its rows from the one table, in its order", func(t *testing.T) {
			m := laid(r.g)
			var labels []string
			for _, item := range m.Rows[0] {
				labels = append(labels, item.Label.Text)
			}
			assert.Equal(t, []string{"attach", "stop", "read"}, labels)
		})
	})
}

// Item 350, the rule of two things that must agree: the key a menu row
// shows is the key its handler answers, because both read one table.
// Every row the table can show is opened alone and its letter pressed.
func TestMenuTable(t *testing.T) {
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	live := state.Session{ID: "live", Title: "Fix the queue", CWD: "/src/kiln", State: state.NeedsYou, LastActivity: at, StartedAt: at.Add(-time.Hour)}

	for c, column := range city.VerbColumns {
		for _, kind := range column.Kinds {
			t.Run("when the "+column.Title+" column shows "+string(kind)+" and the key it shows is pressed", func(t *testing.T) {
				scene := city.NewScene(city.NewLayout())
				scene.Resize(1280, 800)
				scene.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{live}})
				require.True(t, scene.Select("live"))
				g := &Game{scene: scene, actor: &recorder{}, scripted: -1, clicked: -1}
				g.openMenu([]city.ActionKind{kind}, city.Point{X: 300, Y: 300}, c)
				var chosen []city.ActionKind
				g.menu().choose = func(k city.ActionKind) { chosen = append(chosen, k) }
				shown := g.menu().layout(g, 1280, 800).Rows[c][0].Cap.Label.Text
				fields := strings.Fields(shown)
				require.NotEmpty(t, fields, "the row shows no key")
				g.pressScripted(fields[0])
				require.NoError(t, g.handleKeys())
				g.scripted, g.scriptedShift = -1, false

				t.Run("it should run that row's action", func(t *testing.T) {
					assert.Equal(t, []city.ActionKind{kind}, chosen)
				})
			})
		}
	}

	t.Run("when a row's letter is rebound", func(t *testing.T) {
		chords, err := config.FindChords(config.Config{Keys: map[string]string{"menu_stop": "x"}}, runtime.GOOS, KnownKey)
		require.NoError(t, err)
		open := func(t *testing.T) (*Game, *[]city.ActionKind) {
			t.Helper()
			scene := city.NewScene(city.NewLayout())
			scene.Resize(1280, 800)
			scene.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{live}})
			require.True(t, scene.Select("live"))
			g := &Game{scene: scene, actor: &recorder{}, scripted: -1, clicked: -1, chords: &chords}
			g.openMenu([]city.ActionKind{city.ActionStop}, city.Point{X: 300, Y: 300}, 0)
			var chosen []city.ActionKind
			g.menu().choose = func(k city.ActionKind) { chosen = append(chosen, k) }
			return g, &chosen
		}

		t.Run("it should show the new letter", func(t *testing.T) {
			g, _ := open(t)
			assert.Equal(t, "x", g.menu().layout(g, 1280, 800).Rows[0][0].Cap.Label.Text)
		})

		for key, runs := range map[ebiten.Key]bool{ebiten.KeyX: true, ebiten.KeyS: false} {
			t.Run(fmt.Sprintf("and %s is pressed", key), func(t *testing.T) {
				g, chosen := open(t)
				g.scripted = key
				require.NoError(t, g.handleKeys())
				g.scripted = -1

				t.Run(fmt.Sprintf("it should run stop only on the new letter (%v)", runs), func(t *testing.T) {
					assert.Equal(t, runs, len(*chosen) == 1)
				})
			})
		}
	})

	t.Run("when the table is read as a whole", func(t *testing.T) {
		t.Run("it should name each row's binding in its column's scope", func(t *testing.T) {
			scopes := []config.Scope{config.ScopeMenuSession, config.ScopeMenuProject, config.ScopeMenuGroup}
			scopeOf := map[string]config.Scope{}
			for _, b := range config.Bindings {
				scopeOf[b.Name] = b.Scope
			}
			for c, column := range city.VerbColumns {
				for _, kind := range column.Kinds {
					assert.Equal(t, scopes[c], scopeOf[menuKeys[kind]], string(kind))
				}
			}
		})

		// alternates never show together, so they may share a key.
		alternates := map[city.ActionKind]city.ActionKind{
			city.ActionAttach: city.ActionResume, city.ActionResume: city.ActionAttach,
			city.ActionPack: city.ActionUnpack, city.ActionUnpack: city.ActionPack,
			city.ActionStar: city.ActionUnstar, city.ActionUnstar: city.ActionStar,
		}

		t.Run("it should give one key to each column", func(t *testing.T) {
			assert.Len(t, menuColumnKeys, len(city.VerbColumns))
		})

		t.Run("it should give no two kinds that can show together in a column one key", func(t *testing.T) {
			for _, column := range city.VerbColumns {
				seen := map[string]city.ActionKind{}
				for _, kind := range column.Kinds {
					key := menuKeys[kind]
					if other, dup := seen[key]; dup && alternates[kind] != other {
						assert.Failf(t, "two rows share a key", "%s: %s and %s both show %s", column.Title, other, kind, key)
					}
					seen[key] = kind
				}
			}
		})

		t.Run("it should key nothing that no column shows", func(t *testing.T) {
			shown := map[city.ActionKind]bool{}
			for _, column := range city.VerbColumns {
				for _, kind := range column.Kinds {
					shown[kind] = true
				}
			}
			for kind := range menuKeys {
				assert.True(t, shown[kind], string(kind))
			}
		})
	})
}
