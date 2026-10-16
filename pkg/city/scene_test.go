package city_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scene(t *testing.T, sessions ...state.Session) *city.Scene {
	t.Helper()
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	s.SetSnapshot(snapshot(sessions...))
	return s
}

func centreOf(t *testing.T, s *city.Scene, id string) city.Point {
	t.Helper()
	for _, b := range s.City().Buildings() {
		if b.Session.ID == id {
			return s.Camera().WorldToScreen(b.Rect.Center())
		}
	}
	t.Fatalf("no building for session %s", id)
	return city.Point{}
}

func TestScene(t *testing.T) {
	t.Run("when the first snapshot arrives", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))

		t.Run("it should build the city", func(t *testing.T) {
			assert.Len(t, s.City().Districts, 2)
		})

		t.Run("it should fit the camera to it", func(t *testing.T) {
			min := s.Camera().WorldToScreen(s.City().Bounds().Min)
			assert.GreaterOrEqual(t, min.X, 0.0)
			assert.GreaterOrEqual(t, min.Y, 0.0)
		})
	})

	t.Run("when a later snapshot arrives", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Pan(city.Point{X: 40, Y: 40})
		panned := s.Camera().Offset
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working)))

		t.Run("it should keep the camera where the user left it", func(t *testing.T) {
			assert.Equal(t, panned, s.Camera().Offset)
		})

		t.Run("it should take the new session", func(t *testing.T) {
			assert.Len(t, s.City().Buildings(), 2)
		})
	})

	t.Run("when the pointer moves over a building", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou))
		s.PointerMove(centreOf(t, s, "a"))

		t.Run("it should hover that building", func(t *testing.T) {
			require.NotNil(t, s.Hover().Building)
			assert.Equal(t, "a", s.Hover().Building.Session.ID)
		})

		t.Run("it should offer that building's card", func(t *testing.T) {
			card, ok := s.Card()
			require.True(t, ok)
			assert.Equal(t, "Fix the CI queue", card.Title)
		})
	})

	t.Run("when the pointer moves over empty ground", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		// Just outside the district, inside the bank the water keeps clear of.
		d := s.City().Districts[0].Rect
		s.PointerMove(s.Camera().WorldToScreen(city.Point{X: d.Max.X + city.BuildingSize/2, Y: d.Min.Y - city.BuildingSize/2}))

		t.Run("it should hover nothing", func(t *testing.T) {
			assert.Nil(t, s.Hover().Building)
			assert.Nil(t, s.Hover().District)
		})

		t.Run("it should offer no card", func(t *testing.T) {
			_, ok := s.Card()
			assert.False(t, ok)
		})
	})

	t.Run("when the pointer rests on a road", func(t *testing.T) {
		snap := snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working))
		snap.Roads = []state.Road{{From: cinders, To: botropolis, Messages: 3, Files: 2, Sessions: 1,
			Paths: []string{botropolis + "/pkg/city/city.go", botropolis + "/README.md"}}}
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		require.Len(t, s.City().Roads, 1)
		require.Len(t, s.City().Streets, 1)
		path := s.City().Streets[0].Path
		s.PointerMove(s.Camera().WorldToScreen(path[len(path)/2]))

		t.Run("it should hover the road", func(t *testing.T) {
			require.NotNil(t, s.Hover().Road)
			assert.Equal(t, 3, s.Hover().Road.Messages)
		})

		t.Run("it should offer the road's card with its files relative to the far district", func(t *testing.T) {
			card, ok := s.Card()
			require.True(t, ok)
			assert.Equal(t, "cinders -> botropolis", card.Title)
			assert.Contains(t, card.Lines, "traffic  3 msgs, 2 files")
			assert.Contains(t, card.Lines, "files    pkg/city/city.go, README.md")
		})
	})

	t.Run("when the pointer rests on a beam", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.MCPCalls = map[string]int{"atlassian": 3}
		snap := snapshot(a)
		snap.Servers = []state.Server{{Name: "atlassian", Calls: 3, Sessions: 1}}
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		beams := s.City().Beams()
		require.Len(t, beams, 1)
		s.PointerMove(s.Camera().WorldToScreen(city.Point{X: (beams[0].From.X + beams[0].To.X) / 2, Y: (beams[0].From.Y + beams[0].To.Y) / 2}))

		t.Run("it should offer the beam's card", func(t *testing.T) {
			card, ok := s.Card()
			require.True(t, ok)
			assert.Equal(t, "atlassian -> Fix the CI queue", card.Title)
			assert.Contains(t, card.Lines, "calls    3 this session")
		})
	})

	t.Run("when the pointer rests on a power line", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		lines := s.City().PowerLines()
		require.Len(t, lines, 1)
		// A hair past the plant, where the line crosses open plaza rather
		// than a district's floor.
		at := lines[0].From.Add(lines[0].To.Sub(lines[0].From).Scale(0.2))
		s.PointerMove(s.Camera().WorldToScreen(at))

		t.Run("it should offer the line's card in tokens per minute", func(t *testing.T) {
			card, ok := s.Card()
			require.True(t, ok)
			assert.Equal(t, "power -> Fix the CI queue", card.Title)
			assert.Contains(t, card.Lines, "cached   148k/min")
		})
	})

	t.Run("when the pointer is dragged", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		before := s.Camera().Offset
		s.Pan(city.Point{X: 20, Y: -10})

		t.Run("it should move the camera by the drag divided by the zoom", func(t *testing.T) {
			assert.InDelta(t, before.X+20/s.Camera().Zoom, s.Camera().Offset.X, 1e-9)
			assert.InDelta(t, before.Y-10/s.Camera().Zoom, s.Camera().Offset.Y, 1e-9)
		})
	})

	t.Run("when the wheel is turned away from the user", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Camera().Zoom = 1
		before := s.Camera().Zoom
		s.Wheel(city.Point{X: 400, Y: 300}, -1)

		t.Run("it should not be there yet after one tick", func(t *testing.T) {
			s.Animate(1.0 / 30)
			assert.Greater(t, s.Camera().Zoom, city.ZoomSteps[3])
			assert.Less(t, s.Camera().Zoom, before)
		})

		t.Run("it should have zoomed out after a second", func(t *testing.T) {
			s.Animate(1)
			assert.InDelta(t, city.ZoomSteps[3], s.Camera().Zoom, 1e-9)
		})

		t.Run("and turned back", func(t *testing.T) {
			s.Wheel(city.Point{X: 400, Y: 300}, 1)
			s.Animate(1)

			t.Run("it should return to where it was", func(t *testing.T) {
				assert.InDelta(t, before, s.Camera().Zoom, 1e-9)
			})
		})

		t.Run("and motion is reduced", func(t *testing.T) {
			s.SetInstant(true)
			s.Wheel(city.Point{X: 400, Y: 300}, -1)

			t.Run("it should jump at once", func(t *testing.T) {
				assert.InDelta(t, city.ZoomSteps[3], s.Camera().Zoom, 1e-9)
			})
		})
	})

	t.Run("when the wheel has asked for a zoom", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Camera().Zoom = 1
		s.Wheel(city.Point{X: 400, Y: 300}, -1)

		t.Run("it should be easing after one tick", func(t *testing.T) {
			s.Animate(1.0 / 30)
			assert.True(t, s.Easing())
		})

		t.Run("it should have stopped easing once the zoom arrives", func(t *testing.T) {
			s.Animate(1)
			assert.False(t, s.Easing())
		})
	})

	t.Run("when nothing has asked the camera to move", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))

		t.Run("it should not be easing", func(t *testing.T) {
			assert.False(t, s.Easing())
		})
	})

	t.Run("when a building is clicked", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou))
		action := s.Click(centreOf(t, s, "a"))

		// A click used to attach, which opens a terminal — a
		// destructive-by-surprise action for anyone who clicked to look
		// rather than to enter. It selects now, and the card it raises
		// offers attach among the rest.
		t.Run("it should do nothing on its own", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
		})

		t.Run("it should select the building", func(t *testing.T) {
			require.NotNil(t, s.Selected())
			assert.Equal(t, "a", s.Selected().Session.ID)
		})

		t.Run("it should still attach when the selection is activated", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionAttach, SessionID: "a"}, s.Activate())
		})
	})

	t.Run("when empty ground is clicked", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Click(centreOf(t, s, "a"))
		d := s.City().Districts[0].Rect
		action := s.Click(s.Camera().WorldToScreen(city.Point{X: d.Max.X + city.BuildingSize/2, Y: d.Min.Y - city.BuildingSize/2}))

		t.Run("it should ask for nothing", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
		})

		t.Run("it should clear the selection", func(t *testing.T) {
			assert.Nil(t, s.Selected())
		})
	})

	t.Run("when the bank beside a district is clicked", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Click(centreOf(t, s, "a"))
		d := s.City().Districts[0].Rect
		action := s.Click(s.Camera().WorldToScreen(city.Point{X: d.Max.X + city.BuildingSize/2, Y: d.Min.Y - city.BuildingSize/2}))

		t.Run("it should ask for nothing", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
		})
	})

	t.Run("when a parked building is clicked", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Resting))
		action := s.Click(centreOf(t, s, "a"))

		t.Run("it should do nothing on its own, as a live one does not", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
		})

		t.Run("it should offer resume when the selection is activated", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionResume, SessionID: "a"}, s.Activate())
		})
	})

	t.Run("when the window is resized and the camera has not been touched", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Resize(1024, 768)

		t.Run("it should refit the city to the new size", func(t *testing.T) {
			centre := s.Camera().WorldToScreen(s.City().Bounds().Center())
			assert.InDelta(t, (1024-city.LibraryLabelWidth)/2, centre.X, 1e-6)
			assert.InDelta(t, city.LabelHeight+(768-city.LabelHeight-city.FitFooter)/2, centre.Y, 1e-6)
		})
	})

	t.Run("when the window is resized after the user has panned", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Pan(city.Point{X: 50, Y: 50})
		offset := s.Camera().Offset
		s.Resize(1024, 768)

		t.Run("it should leave the camera where the user put it", func(t *testing.T) {
			assert.Equal(t, offset, s.Camera().Offset)
		})

		t.Run("and the user asks to fit", func(t *testing.T) {
			s.Fit()

			t.Run("it should fit again", func(t *testing.T) {
				centre := s.Camera().WorldToScreen(s.City().Bounds().Center())
				assert.InDelta(t, (1024-city.LibraryLabelWidth)/2, centre.X, 1e-6)
			})
		})
	})

	t.Run("when the window is resized before any snapshot", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(1024, 768)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))

		t.Run("it should fit the city to the new size", func(t *testing.T) {
			centre := s.Camera().WorldToScreen(s.City().Bounds().Center())
			assert.InDelta(t, (1024-city.LibraryLabelWidth)/2, centre.X, 1e-6)
			assert.InDelta(t, city.LabelHeight+(768-city.LabelHeight-city.FitFooter)/2, centre.Y, 1e-6)
		})
	})
}

func TestSceneDetail(t *testing.T) {
	s := scene(t, session("a", cinders, state.Working))

	t.Run("when the camera is below the detail zoom", func(t *testing.T) {
		s.Camera().Zoom = city.DetailZoom / 2

		t.Run("it should show the map view", func(t *testing.T) {
			assert.False(t, s.Detailed())
		})

		t.Run("it should hide building titles", func(t *testing.T) {
			assert.False(t, s.TitlesVisible())
		})
	})

	t.Run("when the camera is at the title zoom", func(t *testing.T) {
		s.Camera().Zoom = city.TitleZoom

		t.Run("it should show sprites and titles", func(t *testing.T) {
			assert.True(t, s.Detailed())
			assert.True(t, s.TitlesVisible())
		})
	})

	t.Run("when a district is narrower on screen than its label needs", func(t *testing.T) {
		d := s.City().Districts[0]
		s.Camera().Zoom = (city.DistrictLabelMinWidth - 1) / d.Rect.Width()

		t.Run("it should hide the district's name", func(t *testing.T) {
			assert.False(t, s.DistrictLabelVisible(d))
		})

		t.Run("and it is zoomed in a little", func(t *testing.T) {
			s.Camera().Zoom = city.DistrictLabelMinWidth / d.Rect.Width()

			t.Run("it should show the name", func(t *testing.T) {
				assert.True(t, s.DistrictLabelVisible(d))
			})
		})
	})
}

func TestDistrictLabelAt(t *testing.T) {
	s := scene(t, session("a", cinders, state.Working))
	d := s.City().Districts[0]

	t.Run("when the district's padding has room for a line", func(t *testing.T) {
		s.Camera().Zoom = 1
		at := s.DistrictLabelAt(d, 16, 60)

		t.Run("it should sit on the floor inside the kerb", func(t *testing.T) {
			assert.Greater(t, at.Y, s.Camera().WorldToScreen(d.Rect.Min).Y)
		})
	})

	t.Run("when the district is too small on screen", func(t *testing.T) {
		s.Camera().Zoom = 0.35
		at := s.DistrictLabelAt(d, 16, 60)

		t.Run("it should sit just above the kerb", func(t *testing.T) {
			assert.Less(t, at.Y, s.Camera().WorldToScreen(d.Rect.Min).Y)
		})
	})
}

func TestSceneProjection(t *testing.T) {
	t.Run("when the scene switches to isometric", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		s.SetProjection(city.Isometric)

		t.Run("it should refit so the projected city is on screen", func(t *testing.T) {
			b := s.City().Bounds()
			for _, corner := range city.Isometric.Corners(b) {
				p := s.Camera().WorldToScreen(city.Isometric.Invert(corner))
				assert.GreaterOrEqual(t, p.X, 0.0)
				assert.LessOrEqual(t, p.X, 800.0)
				assert.GreaterOrEqual(t, p.Y, 0.0)
				assert.LessOrEqual(t, p.Y, 600.0)
			}
		})

		t.Run("it should still find a building under the pointer", func(t *testing.T) {
			s.PointerMove(centreOf(t, s, "a"))
			require.NotNil(t, s.Hover().Building)
			assert.Equal(t, "a", s.Hover().Building.Session.ID)
		})

		t.Run("it should read sprites from half zoom", func(t *testing.T) {
			s.Camera().Zoom = city.IsoDetailZoom
			assert.True(t, s.Detailed())
		})

		// Bug 40 moved this from the block's top corner to its bottom
		// one: Aria asked for the plate "at the bottom", and the near
		// vertex is the emptiest part of a block in this projection.
		t.Run("it should put the project's name below its near corner", func(t *testing.T) {
			d := s.City().Districts[0]
			at := s.DistrictLabelAt(d, 16, 60)
			near := s.Camera().WorldToScreen(d.Rect.Max)
			assert.Greater(t, at.Y, near.Y)
		})

		t.Run("it should centre that plate on the corner it hangs from", func(t *testing.T) {
			d := s.City().Districts[0]
			near := s.Camera().WorldToScreen(d.Rect.Max)
			assert.InDelta(t, near.X-30, s.DistrictLabelAt(d, 16, 60).X, 1e-9)
		})

		t.Run("it should reserve no side insets for labels", func(t *testing.T) {
			assert.Zero(t, s.Insets().Left)
			assert.Zero(t, s.Insets().Right)
		})

		t.Run("it should keep the minimap inside its box", func(t *testing.T) {
			box := city.RectAt(600, 500, 160, 96)
			m := s.Minimap(box)
			for _, corner := range city.Isometric.Corners(s.City().Bounds()) {
				assert.True(t, box.Contains(m.Project(city.Isometric.Invert(corner))))
			}
		})
	})
}

func TestJumpTo(t *testing.T) {
	t.Run("when two sessions need you and one is working", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou), session("b", botropolis, state.NeedsYou), session("c", "/p/c", state.Working))
		first := s.JumpTo(state.NeedsYou)
		require.NotNil(t, first)

		t.Run("it should select a building that needs you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, first.Session.State)
			assert.Equal(t, first, s.Selected())
		})

		t.Run("it should centre that building in the view", func(t *testing.T) {
			p := s.Camera().WorldToScreen(first.Rect.Center())
			in := s.Insets()
			assert.InDelta(t, in.Left+(800-in.Left-in.Right)/2, p.X, 1e-6)
			assert.InDelta(t, in.Top+(600-in.Top-in.Bottom)/2, p.Y, 1e-6)
		})

		t.Run("it should be close enough to see sprites", func(t *testing.T) {
			assert.True(t, s.Detailed())
		})

		t.Run("and it jumps again", func(t *testing.T) {
			second := s.JumpTo(state.NeedsYou)

			t.Run("it should move on to the other one", func(t *testing.T) {
				assert.NotEqual(t, first, second)
				assert.Equal(t, state.NeedsYou, second.Session.State)
			})

			t.Run("and again", func(t *testing.T) {
				t.Run("it should wrap round", func(t *testing.T) {
					assert.Equal(t, first, s.JumpTo(state.NeedsYou))
				})
			})
		})

		t.Run("and the window is resized after a jump", func(t *testing.T) {
			offset := s.Camera().Offset
			s.Resize(1024, 768)

			t.Run("it should not refit over the user's position", func(t *testing.T) {
				assert.Equal(t, offset, s.Camera().Offset)
			})
		})
	})

	t.Run("when nothing is in the state", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))

		t.Run("it should go nowhere", func(t *testing.T) {
			assert.Nil(t, s.JumpTo(state.NeedsYou))
			assert.Nil(t, s.Selected())
		})
	})
}

func TestActivate(t *testing.T) {
	t.Run("when a live building was reached by a jump and Enter is pressed", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou))
		require.NotNil(t, s.JumpTo(state.NeedsYou))

		t.Run("it should ask to attach it", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionAttach, SessionID: "a"}, s.Activate())
		})
	})

	t.Run("when a parked building is selected", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Resting))
		s.JumpTo(state.Resting)

		t.Run("it should ask to resume it", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionResume, SessionID: "a"}, s.Activate())
		})
	})

	t.Run("when nothing is selected", func(t *testing.T) {
		t.Run("it should do nothing", func(t *testing.T) {
			assert.Equal(t, city.Action{}, scene(t, session("a", cinders, state.Working)).Activate())
		})
	})
}

func TestSelectionActions(t *testing.T) {
	t.Run("when a live building is selected", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.NeedsYou))
		require.NotNil(t, s.JumpTo(state.NeedsYou))

		t.Run("it should offer the way in, the session, then the project", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionAttach, city.ActionStop, city.ActionRead, city.ActionGenerate, city.ActionPack, city.ActionNew, city.ActionReveal, city.ActionCopyPath, city.ActionHide, city.ActionStar, city.ActionDetails}, s.Actions())
		})

		t.Run("it should ask to stop it by id", func(t *testing.T) {
			action, _ := s.Act(city.ActionStop)
			assert.Equal(t, city.Action{Kind: city.ActionStop, SessionID: "a"}, action)
		})

		t.Run("it should start a new session in its directory", func(t *testing.T) {
			action, _ := s.Act(city.ActionNew)
			assert.Equal(t, city.Action{Kind: city.ActionNew, Dir: cinders}, action)
		})

		t.Run("it should reveal its directory", func(t *testing.T) {
			action, _ := s.Act(city.ActionReveal)
			assert.Equal(t, city.Action{Kind: city.ActionReveal, Dir: cinders}, action)
		})

		t.Run("it should copy its directory", func(t *testing.T) {
			action, note := s.Act(city.ActionCopyPath)
			assert.Equal(t, city.Action{Kind: city.ActionCopyPath, Dir: cinders}, action)
			assert.Equal(t, "copied "+cinders, note)
		})

		t.Run("it should put the actions on the selection's card", func(t *testing.T) {
			card, b, ok := s.SelectedCard()
			require.True(t, ok)
			assert.Equal(t, "a", b.Session.ID)
			assert.Equal(t, "attach", card.Actions[0])
		})
	})

	t.Run("when a parked building is selected", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Resting))
		require.NotNil(t, s.JumpTo(state.Resting))

		t.Run("it should offer resume and no stop", func(t *testing.T) {
			kinds := s.Actions()
			assert.Equal(t, city.ActionResume, kinds[0])
			assert.NotContains(t, kinds, city.ActionStop)
		})
	})

	t.Run("when a project is starred", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		require.NotNil(t, s.JumpTo(state.Working))
		_, note := s.Act(city.ActionStar)

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, "starred cinders", note)
		})

		t.Run("it should remember it in the layout", func(t *testing.T) {
			assert.True(t, s.Layout().IsStarred(cinders))
		})

		t.Run("it should offer unstar next", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionUnstar)
		})

		t.Run("and it is unstarred", func(t *testing.T) {
			s.Act(city.ActionUnstar)

			t.Run("it should forget it", func(t *testing.T) {
				assert.False(t, s.Layout().IsStarred(cinders))
			})
		})
	})

	t.Run("when a project is hidden", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		s.Click(s.Camera().WorldToScreen(s.City().Districts[1].Buildings[0].Rect.Center()))
		require.NotNil(t, s.Selected())
		_, note := s.Act(city.ActionHide)

		t.Run("it should drop its district from the city", func(t *testing.T) {
			require.Len(t, s.City().Districts, 1)
			assert.Equal(t, "botropolis", s.City().Districts[0].Name)
		})

		t.Run("it should say how to get it back", func(t *testing.T) {
			assert.Contains(t, note, "hid cinders")
		})

		t.Run("it should remember it in the layout", func(t *testing.T) {
			assert.Equal(t, []string{cinders}, s.Layout().HiddenRoots())
		})

		t.Run("it should clear the selection", func(t *testing.T) {
			assert.Nil(t, s.Selected())
		})

		t.Run("and it is unhidden", func(t *testing.T) {
			s.Unhide(cinders)

			t.Run("it should come back", func(t *testing.T) {
				assert.Len(t, s.City().Districts, 2)
			})
		})
	})

	t.Run("when c is pressed with a building selected", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		require.NotNil(t, s.JumpTo(state.Working))
		action, _ := s.NewHere()

		t.Run("it should start a session in that directory", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionNew, Dir: cinders}, action)
		})
	})

	t.Run("when c is pressed over a district with nothing selected", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.PointerMove(s.Camera().WorldToScreen(s.City().Districts[0].Rect.Min.Add(city.Point{X: 2, Y: 2})))
		action, _ := s.NewHere()

		t.Run("it should start a session in the district's root", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionNew, Dir: cinders}, action)
		})
	})

	t.Run("when c is pressed over nothing", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		action, note := s.NewHere()

		t.Run("it should do nothing and say why", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
			assert.Contains(t, note, "select")
		})
	})
}

func TestSelectByID(t *testing.T) {
	t.Run("when a session is selected by id", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		before := s.Camera().Offset
		found := s.Select("b")

		t.Run("it should find it", func(t *testing.T) {
			assert.True(t, found)
		})

		t.Run("it should select it", func(t *testing.T) {
			require.NotNil(t, s.Selected())
			assert.Equal(t, "b", s.Selected().Session.ID)
		})

		t.Run("it should move the camera to it", func(t *testing.T) {
			assert.NotEqual(t, before, s.Camera().Offset)
		})
	})

	t.Run("when the id is not on the map", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))

		t.Run("it should say so and keep the selection", func(t *testing.T) {
			assert.False(t, s.Select("zzz"))
			assert.Nil(t, s.Selected())
		})
	})

	t.Run("when the sidebar takes the left edge", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.SetLeftChrome(288)

		t.Run("it should add it to the left inset", func(t *testing.T) {
			assert.InDelta(t, 288, s.Insets().Left, 1e-9)
		})
	})
}

func TestCelebration(t *testing.T) {
	merged := func(id string, st string) state.Session {
		s := session(id, cinders, state.Working)
		s.PRs = []claude.PR{{Number: 1181, URL: "https://github.com/mCedar/mullet/pull/1181", Repository: "mCedar/mullet", State: st}}
		return s
	}
	clock := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)
	tick := func(s *city.Scene, by time.Duration) {
		clock = clock.Add(by)
		s.SetClock(func() time.Time { return clock }, time.UTC)
	}

	t.Run("when a session's PR merges between snapshots", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		tick(s, 0)
		s.SetSnapshot(snapshot(merged("a", claude.PROpen)))
		s.SetSnapshot(snapshot(merged("a", claude.PRMerged)))

		t.Run("it should start celebrating", func(t *testing.T) {
			assert.InDelta(t, 0, s.Celebration("a"), 1e-9)
		})

		t.Run("it should count the merge on the building", func(t *testing.T) {
			assert.Equal(t, 1, s.City().Buildings()[0].Merged)
		})

		t.Run("and a second passes", func(t *testing.T) {
			tick(s, time.Second)

			t.Run("it should be part way through", func(t *testing.T) {
				p := s.Celebration("a")
				assert.Greater(t, p, 0.3)
				assert.Less(t, p, 0.5)
			})
		})

		t.Run("and the show is over", func(t *testing.T) {
			tick(s, 3*time.Second)

			t.Run("it should stop", func(t *testing.T) {
				assert.Equal(t, -1.0, s.Celebration("a"))
			})
		})
	})

	t.Run("when the first snapshot already has a merged PR", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(merged("a", claude.PRMerged)))

		t.Run("it should not celebrate an old merge", func(t *testing.T) {
			assert.Equal(t, -1.0, s.Celebration("a"))
		})
	})
}

func TestSceneEvents(t *testing.T) {
	clock := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)
	s := city.NewScene(city.NewLayout())
	s.Resize(800, 600)
	s.SetClock(func() time.Time { return clock }, time.UTC)
	s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
	s.AddEvents([]events.Event{{At: clock.Add(time.Minute), Kind: events.NeedsYou, SessionID: "a", Title: "Fix the CI queue"}})
	left := clock.Add(time.Minute)
	s.AddEvents([]events.Event{{At: clock.Add(2 * time.Minute), Kind: events.Error, SessionID: "a", Title: "Fix the CI queue", Detail: "1 api errors"}})

	t.Run("when events have been handed in", func(t *testing.T) {
		t.Run("it should log what happened, newest first", func(t *testing.T) {
			logged := s.Events()
			require.Len(t, logged, 2)
			assert.Equal(t, events.Error, logged[0].Kind)
			assert.Equal(t, events.NeedsYou, logged[1].Kind)
		})

		t.Run("it should say what needed you since you left", func(t *testing.T) {
			away := s.Away(left)
			require.Len(t, away, 1)
			assert.Equal(t, events.Error, away[0].Kind)
		})
	})
}

func TestNamePlates(t *testing.T) {
	t.Run("when a district has awake sessions", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Wheel(s.Size().Scale(0.5), 3)
		s.Animate(1)

		t.Run("it should show its plate", func(t *testing.T) {
			assert.True(t, s.DistrictLabelVisible(s.City().Districts[0]))
		})
	})

	// Bug 40 reversed this. A plate used to wait for the district to be
	// busy, hovered or selected, so the labels you needed in order to
	// find your way were the ones that disappeared when the city went
	// quiet. Aria's ruling is that a project's name is permanent and a
	// session's title is the hover-only one.
	t.Run("when a district holds only resting sessions", func(t *testing.T) {
		s := scene(t, session("b", cinders, state.Resting))
		s.Wheel(s.Size().Scale(0.5), 3)
		s.Animate(1)

		t.Run("it should still show its plate, with nothing happening in it", func(t *testing.T) {
			assert.True(t, s.DistrictLabelVisible(s.City().Districts[0]))
		})
	})

	t.Run("when a district is too narrow on screen to read", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Camera().Zoom = 0.01

		t.Run("it should drop its plate rather than crowd the map", func(t *testing.T) {
			assert.False(t, s.DistrictLabelVisible(s.City().Districts[0]))
		})
	})
}

func TestWaterCard(t *testing.T) {
	t.Run("when the pointer rests on the river", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		require.NotEmpty(t, s.City().RiverCells)
		at := s.Camera().WorldToScreen(s.City().RiverCells[len(s.City().RiverCells)/2].Cell.Center())
		s.PointerMove(at)
		card, ok := s.Card()
		require.True(t, ok)

		t.Run("it should say it runs out to the sea", func(t *testing.T) {
			assert.Equal(t, "river", card.Title)
			assert.Contains(t, card.Lines, "runs through the island to the sea")
		})

		t.Run("it should not be clickable", func(t *testing.T) {
			assert.Equal(t, city.Action{}, s.Click(at))
		})
	})

	t.Run("when the pointer rests on a park block", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		require.NotEmpty(t, s.City().Parks)
		at := s.Camera().WorldToScreen(s.City().Parks[0].Rect.Center())
		s.PointerMove(at)
		_, ok := s.Card()

		t.Run("it should show no card, because the ground means nothing", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when the pointer rests on the fountain", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.PointerMove(s.Camera().WorldToScreen(s.City().Fountain.Center()))
		card, ok := s.Card()
		require.True(t, ok)

		t.Run("it should name the fountain", func(t *testing.T) {
			assert.Equal(t, "fountain", card.Title)
		})
	})
}

func TestCycleLight(t *testing.T) {
	t.Run("when nothing runs unattended and the light is cycled", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		require.False(t, s.City().Night)
		first := s.CycleLight()

		t.Run("it should be night", func(t *testing.T) {
			assert.Equal(t, city.LightNight, first)
			assert.True(t, s.City().Night)
		})

		t.Run("and a new snapshot arrives", func(t *testing.T) {
			s.SetSnapshot(snapshot(session("a", cinders, state.Working)))

			t.Run("it should stay night", func(t *testing.T) {
				assert.True(t, s.City().Night)
			})
		})

		t.Run("and it is cycled again", func(t *testing.T) {
			second := s.CycleLight()

			t.Run("it should be day", func(t *testing.T) {
				assert.Equal(t, city.LightDay, second)
				assert.False(t, s.City().Night)
			})
		})

		t.Run("and once more", func(t *testing.T) {
			third := s.CycleLight()
			s.SetSnapshot(snapshot(session("a", cinders, state.Working)))

			t.Run("it should follow the sessions again", func(t *testing.T) {
				assert.Equal(t, city.LightAuto, third)
				assert.False(t, s.City().Night)
			})
		})
	})

	t.Run("when it is evening on the clock", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.SetClock(func() time.Time { return time.Date(2026, 9, 18, 22, 30, 0, 0, time.UTC) }, time.UTC)

		t.Run("it should be night", func(t *testing.T) {
			assert.True(t, s.City().Night)
		})

		t.Run("and day is forced", func(t *testing.T) {
			s.CycleLight()
			s.CycleLight()

			t.Run("it should be day regardless", func(t *testing.T) {
				assert.False(t, s.City().Night)
			})
		})
	})

	t.Run("when it is afternoon and a session runs unattended", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Unattended))
		s.SetClock(func() time.Time { return time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC) }, time.UTC)

		t.Run("it should stay day", func(t *testing.T) {
			assert.False(t, s.City().Night)
		})

		t.Run("and the clock is scrubbed eight hours on", func(t *testing.T) {
			s.Scrub(8 * time.Hour)

			t.Run("it should be night", func(t *testing.T) {
				assert.True(t, s.City().Night)
			})

			t.Run("it should read 22:00", func(t *testing.T) {
				assert.Equal(t, 22, s.Clock().Hour())
			})
		})

		t.Run("and the scrub is cleared", func(t *testing.T) {
			s.Scrub(0)

			t.Run("it should be live and day again", func(t *testing.T) {
				assert.False(t, s.City().Night)
				assert.Equal(t, 14, s.Clock().Hour())
			})
		})
	})
}

func TestTopChrome(t *testing.T) {
	t.Run("when the window has a strip along the top", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.SetTopChrome(26)

		t.Run("it should add the strip to the top inset", func(t *testing.T) {
			assert.InDelta(t, city.LabelHeight+26, s.Insets().Top, 1e-9)
		})
	})
}

func TestCameraClamp(t *testing.T) {
	t.Run("when the camera is panned far past the edge", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Pan(city.Point{X: 100_000, Y: -100_000})
		centre := s.Camera().ScreenToWorld(s.Size().Scale(0.5))
		plane := s.Camera().Projection.Bounds(s.City().Island().Reach())

		t.Run("it should keep the island or its shallows under the window's centre", func(t *testing.T) {
			assert.True(t, plane.Inset(-1e-6).Contains(s.Camera().Projection.Apply(centre)), centre)
		})
	})

	t.Run("when the camera is panned a little", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		before := s.Camera().Offset
		s.Pan(city.Point{X: 10, Y: 10})

		t.Run("it should move freely", func(t *testing.T) {
			assert.NotEqual(t, before, s.Camera().Offset)
		})
	})
}

func TestSceneSize(t *testing.T) {
	t.Run("when the scene is resized", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.Resize(640, 480)

		t.Run("it should report the new size", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 640, Y: 480}, s.Size())
		})
	})
}

func TestBottomChrome(t *testing.T) {
	t.Run("when the window has a footer taller than the fit margin", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.SetBottomChrome(city.FitFooter + 30)

		t.Run("it should reserve the footer at the bottom", func(t *testing.T) {
			assert.InDelta(t, city.FitFooter+30, s.Insets().Bottom, 1e-9)
		})
	})

	t.Run("when the footer is shorter than the fit margin", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		s.SetBottomChrome(10)

		t.Run("it should keep the fit margin", func(t *testing.T) {
			assert.InDelta(t, city.FitFooter, s.Insets().Bottom, 1e-9)
		})
	})
}

func TestMinimap(t *testing.T) {
	t.Run("when the city is projected into a box", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		box := city.RectAt(600, 500, 160, 96)
		m := s.Minimap(box)
		bounds := s.City().Bounds()

		t.Run("it should keep the whole city inside the box", func(t *testing.T) {
			assert.True(t, box.Contains(m.Project(bounds.Min)))
			assert.True(t, box.Contains(m.Project(bounds.Max)))
		})

		t.Run("it should keep the city's proportions", func(t *testing.T) {
			r := m.ProjectRect(bounds)
			assert.InDelta(t, bounds.Width()/bounds.Height(), r.Width()/r.Height(), 1e-9)
		})

		t.Run("it should mark the fitted camera as seeing the whole city", func(t *testing.T) {
			r := m.ProjectRect(bounds)
			assert.LessOrEqual(t, m.View.Min.X, r.Min.X+1e-9)
			assert.GreaterOrEqual(t, m.View.Max.X, r.Max.X-1e-9)
		})
	})

	t.Run("when there is no city", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		m := s.Minimap(city.RectAt(0, 0, 160, 96))

		t.Run("it should have nothing to scale", func(t *testing.T) {
			assert.Zero(t, m.Scale)
		})
	})
}

func TestDemolish(t *testing.T) {
	t.Run("when nothing is selected", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Resting))
		action, note := s.Demolish(now)

		t.Run("it should do nothing and say why", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
			assert.Contains(t, note, "select")
		})
	})

	t.Run("when d is pressed once on a selected building", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Resting))
		s.Click(centreOf(t, s, "a"))
		action, note := s.Demolish(now)

		t.Run("it should only arm", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
			assert.Contains(t, note, "press d again")
		})

		t.Run("and d is pressed again in time", func(t *testing.T) {
			action, _ := s.Demolish(now.Add(2 * time.Second))

			t.Run("it should demolish", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionDemolish, SessionID: "a"}, action)
			})
		})
	})

	t.Run("when the second press comes too late", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Resting))
		s.Click(centreOf(t, s, "a"))
		s.Demolish(now)
		action, note := s.Demolish(now.Add(10 * time.Second))

		t.Run("it should re-arm instead", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
			assert.Contains(t, note, "press d again")
		})
	})

	t.Run("when the selection changes between presses", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Resting), session("b", cinders, state.Resting))
		s.Click(centreOf(t, s, "a"))
		s.Demolish(now)
		s.Click(centreOf(t, s, "b"))
		action, _ := s.Demolish(now.Add(time.Second))

		t.Run("it should not demolish the new selection on the first press", func(t *testing.T) {
			assert.Equal(t, city.Action{}, action)
		})
	})
}

func TestSceneInsets(t *testing.T) {
	t.Run("when the city has towers", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetSnapshot(richSnapshot())

		t.Run("it should reserve label room on the left and top", func(t *testing.T) {
			assert.Equal(t, city.Insets{Left: float64(len("datadog-mcp"))*city.LabelCharWidth + city.TowerLabelMargin, Right: city.LibraryLabelWidth, Top: city.LabelHeight, Bottom: city.FitFooter}, s.Insets())
		})

		t.Run("it should keep the first tower clear of the label column", func(t *testing.T) {
			assert.GreaterOrEqual(t, s.Camera().WorldToScreen(s.City().Towers[0].Rect.Min).X, s.City().TowerLabelWidth())
		})
	})

	t.Run("when the window is too small for labels", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(500, 300)
		var sessions []state.Session
		for i := 0; i < 40; i++ {
			sessions = append(sessions, session(fmt.Sprintf("s%02d", i), fmt.Sprintf("/p/%02d", i), state.Working))
		}
		snap := snapshot(sessions...)
		snap.Servers = []state.Server{{Name: "atlassian"}}
		s.SetSnapshot(snap)

		t.Run("it should hide the labels", func(t *testing.T) {
			assert.False(t, s.LabelsVisible())
		})

		t.Run("it should fit without the label room", func(t *testing.T) {
			plain := city.NewCamera()
			plain.FitWithInsets(s.City().Extent(), 500, 300, city.Insets{Bottom: city.FitFooter})
			assert.InDelta(t, plain.Zoom, s.Camera().Zoom, 1e-9)
			assert.GreaterOrEqual(t, s.Camera().WorldToScreen(s.City().Bounds().Min).X, 0.0)
		})
	})

	t.Run("when the city has no towers", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))

		t.Run("it should reserve no left inset", func(t *testing.T) {
			assert.Zero(t, s.Insets().Left)
		})
	})
}

func TestSidebarRoom(t *testing.T) {
	const panel = 288.0
	leftmost := func(s *city.Scene) float64 {
		extent := s.City().Extent()
		left := math.Inf(1)
		for _, corner := range []city.Point{extent.Min, extent.Max, {X: extent.Min.X, Y: extent.Max.Y}, {X: extent.Max.X, Y: extent.Min.Y}} {
			left = math.Min(left, s.Camera().WorldToScreen(corner).X)
		}
		return left
	}

	t.Run("when the sidebar opens on an untouched fit view", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		require.Less(t, leftmost(s), panel)
		s.SetLeftChrome(panel)

		t.Run("it should refit the city clear of the panel", func(t *testing.T) {
			assert.GreaterOrEqual(t, leftmost(s), panel-1e-6)
		})
	})

	t.Run("when the sidebar opens after the view was panned", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		s.Pan(city.Point{X: 40, Y: 0})
		before := centreOf(t, s, "a")
		s.SetLeftChrome(panel)

		t.Run("it should slide the view right by the panel's width", func(t *testing.T) {
			assert.InDelta(t, before.X+panel, centreOf(t, s, "a").X, 1e-6)
		})
	})

	t.Run("when the sidebar closes again on an untouched view", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		before := centreOf(t, s, "a")
		s.SetLeftChrome(panel)
		s.SetLeftChrome(0)

		t.Run("it should give the room back", func(t *testing.T) {
			assert.InDelta(t, before.X, centreOf(t, s, "a").X, 1e-6)
		})
	})

	t.Run("when the city is too big for labels at fit and the sidebar opens", func(t *testing.T) {
		s := scene(t, append(sessionsIn(cinders, 40), sessionsIn(botropolis, 40)...)...)
		s.Resize(360, 240)
		require.False(t, s.LabelsVisible())
		s.SetLeftChrome(120)

		t.Run("it should still keep the city clear of the panel", func(t *testing.T) {
			assert.GreaterOrEqual(t, leftmost(s), 120-1e-6)
		})
	})

	t.Run("when the strip's height arrives after the first fit", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		before := centreOf(t, s, "a")
		s.SetTopChrome(26)

		t.Run("it should refit the city below it", func(t *testing.T) {
			assert.InDelta(t, before.Y+13, centreOf(t, s, "a").Y, 1e-6)
		})
	})
}

func TestLandmarkLabels(t *testing.T) {
	t.Run("when the view is at fit", func(t *testing.T) {
		s := scene(t, append(sessionsIn(cinders, 6), sessionsIn(botropolis, 6)...)...)
		require.Less(t, s.Camera().Zoom, city.LandmarkLabelZoom)

		t.Run("it should hide the plaza landmarks' plates", func(t *testing.T) {
			assert.False(t, s.LandmarkLabelsVisible())
		})
	})

	t.Run("when the view is zoomed in past the landmark label zoom", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))
		for s.Camera().Zoom < city.LandmarkLabelZoom {
			s.Wheel(city.Point{X: 400, Y: 300}, 1)
			s.Animate(10)
		}

		t.Run("it should show them", func(t *testing.T) {
			assert.True(t, s.LandmarkLabelsVisible())
		})
	})
}

func TestVoyages(t *testing.T) {
	clock := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)
	newScene := func() *city.Scene {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		return s
	}

	t.Run("when the first snapshot lands", func(t *testing.T) {
		s := newScene()
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))

		t.Run("it should start no voyage", func(t *testing.T) {
			assert.Empty(t, s.Voyages())
		})

		t.Run("it should have every building risen", func(t *testing.T) {
			assert.InDelta(t, 1, s.Rising("a"), 1e-9)
		})
	})

	t.Run("when a live session appears afterwards", func(t *testing.T) {
		s := newScene()
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
		clock = clock.Add(time.Minute)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), session("b", cinders, state.Working)))
		voyages := s.Voyages()
		require.Len(t, voyages, 1)
		v := voyages[0]

		t.Run("it should sail an arrival for the new session", func(t *testing.T) {
			assert.Equal(t, city.Arrival, v.Kind)
			assert.Equal(t, "b", v.SessionID)
		})

		// The river is plan.RiverCols wide now, so a tug sails its
		// centre line rather than the middle of its western column.
		t.Run("it should come in from the sea at the river's north mouth", func(t *testing.T) {
			north, _ := s.City().Island().Mouths()
			assert.Equal(t, north, v.From)
		})

		t.Run("it should dock beside the session's district", func(t *testing.T) {
			d := s.City().Districts[0]
			north, _, _, ok := s.City().RiverBand()
			require.True(t, ok)
			assert.InDelta(t, north.X, v.To.X, 1e-9)
			assert.InDelta(t, d.Rect.Center().Y, v.To.Y, 1e-9)
		})

		t.Run("it should hold the building down until it docks", func(t *testing.T) {
			assert.Zero(t, s.Rising("b"))
			assert.InDelta(t, 1, s.Rising("a"), 1e-9)
		})

		t.Run("and it has sailed its time", func(t *testing.T) {
			clock = clock.Add(city.SailFor + city.RiseFor/2)

			t.Run("it should be rising", func(t *testing.T) {
				r := s.Rising("b")
				assert.Greater(t, r, 0.0)
				assert.Less(t, r, 1.0)
			})

			t.Run("and it has risen", func(t *testing.T) {
				clock = clock.Add(city.RiseFor)

				t.Run("it should stand and the voyage be over", func(t *testing.T) {
					assert.InDelta(t, 1, s.Rising("b"), 1e-9)
					assert.Empty(t, s.Voyages())
				})
			})
		})
	})

	t.Run("when a live session disappears", func(t *testing.T) {
		s := newScene()
		gone := session("b", botropolis, state.Working)
		gone.Title = "Symbol meaning"
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), gone))
		clock = clock.Add(time.Minute)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
		voyages := s.Voyages()
		require.Len(t, voyages, 1)
		v := voyages[0]

		t.Run("it should sail a departure carrying the title", func(t *testing.T) {
			assert.Equal(t, city.Departure, v.Kind)
			assert.Equal(t, "Symbol meaning", v.Title)
		})

		t.Run("it should leave downriver and out to sea", func(t *testing.T) {
			_, south := s.City().Island().Mouths()
			assert.Equal(t, south, v.To)
		})

		t.Run("it should be named on its card", func(t *testing.T) {
			assert.Equal(t, "leaving: Symbol meaning", v.Card().Title)
		})
	})
}

func TestSceneCardDetails(t *testing.T) {
	selected := func(t *testing.T, sessions ...state.Session) *city.Scene {
		t.Helper()
		s := scene(t, sessions...)
		s.Click(centreOf(t, s, sessions[0].ID))
		require.NotNil(t, s.Selected())
		return s
	}
	recapped := session("a", cinders, state.NeedsYou)
	recapped.Recap = claude.Recap{Text: "We shipped the button.", At: now.Add(-2 * time.Hour), PromptsSince: 3}

	t.Run("when a building is selected", func(t *testing.T) {
		s := selected(t, recapped, session("b", botropolis, state.Working))

		t.Run("it should offer details last", func(t *testing.T) {
			kinds := s.Actions()
			assert.Equal(t, city.ActionDetails, kinds[len(kinds)-1])
		})

		t.Run("and details is pressed", func(t *testing.T) {
			s.Act(city.ActionDetails)
			card, _, _ := s.SelectedCard()

			t.Run("it should fold the table out", func(t *testing.T) {
				assert.True(t, card.Open)
			})

			t.Run("it should offer hide details", func(t *testing.T) {
				assert.Contains(t, s.Actions(), city.ActionDetailsHide)
			})
		})

		t.Run("and another building is selected", func(t *testing.T) {
			s.Act(city.ActionDetails)
			s.Click(centreOf(t, s, "b"))
			card, _, _ := s.SelectedCard()

			t.Run("it should fold details away", func(t *testing.T) {
				assert.False(t, card.Open)
			})
		})
	})

	t.Run("when no recap and nothing generating", func(t *testing.T) {
		s := selected(t, session("a", cinders, state.NeedsYou))

		t.Run("it should offer generate summary", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionGenerate)
		})
	})
}

func TestSceneLadder(t *testing.T) {
	quiet := session("a", cinders, state.Resting)
	quiet.LastActivity = now.Add(-4 * 24 * time.Hour)

	t.Run("when the scene is new", func(t *testing.T) {
		s := scene(t, quiet)

		t.Run("it should climb by the default thresholds", func(t *testing.T) {
			assert.Equal(t, state.RungResting, s.City().Buildings()[0].Rung)
		})
	})

	t.Run("when the thresholds change", func(t *testing.T) {
		s := scene(t, quiet)
		s.SetLadder(state.Thresholds{Shuttered: 3 * 24 * time.Hour, Overgrown: 10 * 24 * time.Hour})

		t.Run("it should climb by the new ones at once", func(t *testing.T) {
			assert.Equal(t, state.RungShuttered, s.City().Buildings()[0].Rung)
		})
	})
}
