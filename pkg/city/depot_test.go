package city_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86, slice 3: a packed-up session leaves its district for the
// depot at the city's edge, and comes back to its old lot when unpacked.
func TestDepot(t *testing.T) {
	day := 24 * time.Hour
	packed := func(id, cwd string, quiet time.Duration) state.Session {
		s := session(id, cwd, state.Resting)
		s.LastActivity = now.Add(-quiet)
		s.Packed, s.PackedAt = true, now.Add(-time.Hour)
		return s
	}
	depotOf := func(t *testing.T, c *city.City) *city.District {
		t.Helper()
		for _, d := range c.Districts {
			if d.Depot {
				return d
			}
		}
		require.FailNow(t, "the city has no depot")
		return nil
	}
	in := func(d *city.District, id string) *city.Building {
		for _, b := range d.Buildings {
			if b.Session.ID == id {
				return b
			}
		}
		return nil
	}

	t.Run("when a session is packed up", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working), packed("b", cinders, day), packed("c", botropolis, day))
		var home *city.District
		for _, d := range c.Districts {
			if d.Root == cinders {
				home = d
			}
		}
		require.NotNil(t, home)

		t.Run("it should leave its district", func(t *testing.T) {
			assert.Nil(t, in(home, "b"))
		})

		t.Run("and its district has nothing else", func(t *testing.T) {
			t.Run("the district should leave the map", func(t *testing.T) {
				for _, d := range c.Districts {
					assert.NotEqual(t, botropolis, d.Root)
				}
			})
		})

		t.Run("it should stand as a crate in the depot", func(t *testing.T) {
			assert.NotNil(t, in(depotOf(t, c), "b"))
		})

		t.Run("it should stand no tree on the depot", func(t *testing.T) {
			for _, tree := range c.Trees {
				assert.False(t, depotOf(t, c).Rect.Contains(tree.At), "a tree at %v", tree.At)
			}
		})

		t.Run("it should stand the depot at the city's edge, beyond every district", func(t *testing.T) {
			assert.Greater(t, depotOf(t, c).Rect.Min.Y, home.Rect.Max.Y)
		})
	})

	t.Run("when crates are in the depot", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working),
			packed("new", cinders, 2*day), packed("old", cinders, 12*day), packed("mid", cinders, 5*day), packed("other", botropolis, 3*day))
		depot := depotOf(t, c)

		t.Run("it should group them by district, in rows", func(t *testing.T) {
			assert.Equal(t, in(depot, "new").Rect.Min.Y, in(depot, "old").Rect.Min.Y)
			assert.NotEqual(t, in(depot, "new").Rect.Min.Y, in(depot, "other").Rect.Min.Y)
		})

		t.Run("it should order each row oldest first", func(t *testing.T) {
			assert.Less(t, in(depot, "old").Rect.Min.X, in(depot, "mid").Rect.Min.X)
			assert.Less(t, in(depot, "mid").Rect.Min.X, in(depot, "new").Rect.Min.X)
		})

		t.Run("the depot's sign should name it and its crates", func(t *testing.T) {
			assert.Equal(t, "depot · 4 crates", depot.Sign().Name)
		})

		t.Run("the depot's sign should carry no move", func(t *testing.T) {
			assert.Empty(t, depot.Sign().Move)
		})

		t.Run("its card should list each district's count and its oldest", func(t *testing.T) {
			assert.Contains(t, depot.Card().Details, "cinders     3  oldest 12 days")
		})
	})

	t.Run("when a packed session is unpacked", func(t *testing.T) {
		layout := city.NewLayout()
		a, b := session("a", cinders, state.Working), session("b", cinders, state.Resting)
		before := buildingIn(t, build(t, layout, a, b), "b").Rect
		b.Packed = true
		build(t, layout, a, b)
		b.Packed = false
		after := build(t, layout, a, b)

		t.Run("it should return to its old lot", func(t *testing.T) {
			assert.Equal(t, before, buildingIn(t, after, "b").Rect)
		})
	})
}

func TestPackActions(t *testing.T) {
	quiet := session("b", cinders, state.Resting)
	crated := quiet
	crated.Packed = true
	selected := func(t *testing.T, id string, sessions ...state.Session) *city.Scene {
		t.Helper()
		s := scene(t, sessions...)
		s.Click(centreOf(t, s, id))
		require.NotNil(t, s.Selected())
		return s
	}

	t.Run("when a building is selected", func(t *testing.T) {
		s := selected(t, "b", session("a", cinders, state.Working), quiet)

		t.Run("the card should offer Pack up", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionPack)
		})

		t.Run("Shift+P should pack it up", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionPack, SessionID: "b"}, s.TogglePack())
		})
	})

	t.Run("when a crate is selected", func(t *testing.T) {
		s := selected(t, "b", session("a", cinders, state.Working), crated)

		t.Run("the card should offer Unpack", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionUnpack)
		})

		t.Run("the card should not offer Pack up", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionPack)
		})

		t.Run("Shift+P should unpack it", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionUnpack, SessionID: "b"}, s.TogglePack())
		})
	})

	t.Run("when the actions are described", func(t *testing.T) {
		for kind, text := range map[city.ActionKind]string{
			city.ActionPack:   "Takes this session off the map and keeps it in the depot. Unpack brings it back.",
			city.ActionUnpack: "Brings this session back to its old lot.",
		} {
			t.Run("each should carry its plain description: "+string(kind), func(t *testing.T) {
				assert.Equal(t, text, kind.Describe())
			})
		}
	})

	t.Run("when d is pressed in the triage walk", func(t *testing.T) {
		waiting := session("w", cinders, state.NeedsYou)
		s := scene(t, waiting)
		s.StartTriage()

		t.Run("triage d should pack the session up", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionPack, SessionID: "w"}, s.TriageDone())
		})
	})
}

// Item 278: the card's pack and unpack did nothing, so a boarded-up
// building stayed boarded up when you packed it.
func TestActPack(t *testing.T) {
	shuttered := session("b", cinders, state.Resting)
	shuttered.LastActivity = now.Add(-8 * 24 * time.Hour)
	crated := shuttered
	crated.Packed, crated.PackedAt = true, now.Add(-time.Hour)
	pressed := func(t *testing.T, kind city.ActionKind, b state.Session) (city.Action, string) {
		t.Helper()
		s := scene(t, session("a", cinders, state.Working), b)
		s.Click(centreOf(t, s, b.ID))
		require.NotNil(t, s.Selected())
		return s.Act(kind)
	}

	t.Run("when a shuttered building is selected", func(t *testing.T) {
		t.Run("and its card's pack is pressed", func(t *testing.T) {
			action, note := pressed(t, city.ActionPack, shuttered)

			t.Run("it should ask the actor to pack that session", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionPack, SessionID: "b"}, action)
			})

			t.Run("it should say it is packing it up", func(t *testing.T) {
				assert.Equal(t, "packing Fix the CI queue up", note)
			})
		})
	})

	t.Run("when a crate is selected", func(t *testing.T) {
		t.Run("and its card's unpack is pressed", func(t *testing.T) {
			action, note := pressed(t, city.ActionUnpack, crated)

			t.Run("it should ask the actor to unpack it", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionUnpack, SessionID: "b"}, action)
			})

			t.Run("it should say it is unpacking it", func(t *testing.T) {
				assert.Equal(t, "unpacking Fix the CI queue", note)
			})
		})
	})

	t.Run("when a live building is selected", func(t *testing.T) {
		t.Run("and its card's pack is pressed", func(t *testing.T) {
			_, note := pressed(t, city.ActionPack, session("b", cinders, state.NeedsYou))

			t.Run("it should say it comes back when it works again", func(t *testing.T) {
				assert.Equal(t, "packed Fix the CI queue; it comes back when it works again", note)
			})
		})
	})
}

// Item 278: a card button that asks for nothing, says nothing and
// changes nothing reads as a broken button, and pack was one.
func TestCardActionsDoSomething(t *testing.T) {
	quiet := func(s state.Session, idle time.Duration) state.Session {
		s.LastActivity = now.Add(-idle)
		return s
	}
	crated := quiet(session("b", cinders, state.Resting), 8*24*time.Hour)
	crated.Packed, crated.PackedAt = true, now.Add(-time.Hour)
	for name, b := range map[string]state.Session{
		"working":   session("b", cinders, state.Working),
		"needs you": session("b", cinders, state.NeedsYou),
		"resting":   quiet(session("b", cinders, state.Resting), time.Hour),
		"shuttered": quiet(session("b", cinders, state.Resting), 8*24*time.Hour),
		"overgrown": quiet(session("b", cinders, state.Resting), 40*24*time.Hour),
		"packed":    crated,
	} {
		selected := func(t *testing.T) *city.Scene {
			t.Helper()
			s := scene(t, session("a", cinders, state.Working), b)
			s.Click(centreOf(t, s, "b"))
			require.NotNil(t, s.Selected())
			return s
		}
		t.Run("when a "+name+" building's card is open", func(t *testing.T) {
			for _, kind := range selected(t).Actions() {
				t.Run("and "+string(kind)+" is pressed", func(t *testing.T) {
					s := selected(t)
					before := s.Actions()
					action, note := s.Act(kind)

					t.Run("it should ask the actor for something, say what happened or change the card", func(t *testing.T) {
						assert.True(t, action.Kind != city.ActionNone || note != "" || !slices.Equal(before, s.Actions()))
					})
				})
			}
		})
	}
}
