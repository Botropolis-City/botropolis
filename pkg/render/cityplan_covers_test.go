package render

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 112: the City Plan lays its badges, compass and cartouche out
// clear of the chrome lying over it, and a badge's hover card is raised
// beside the badge rather than in a corner.
func TestCityPlanCovers(t *testing.T) {
	const width, height = 1600.0, 1000.0
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	require.NoError(t, err)
	newGame := func(t *testing.T, chip bool) *Game {
		t.Helper()
		var sessions []state.Session
		for i := 0; i < 40; i++ {
			cwd := fmt.Sprintf("/p/d%02d", i)
			sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at})
		}
		layout := city.NewLayout()
		if chip {
			layout.Lenses = map[string]int{city.LensAttention.Name(): city.LensPanelViewings + 1}
		}
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetProjection(city.Isometric)
		s.SetInstant(true)
		s.Resize(width, height)
		s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
		s.ToggleCityPlan()
		return &Game{scene: s, theme: th, faces: f, scripted: -1, clicked: -1}
	}

	for _, c := range []struct {
		name   string
		chip   bool
		stream bool
	}{
		{"the Attention panel in full", false, false},
		{"the Attention panel as a chip", true, false},
		{"the stream shown in its corner", true, true},
	} {
		t.Run("when the plan is open under "+c.name, func(t *testing.T) {
			g := newGame(t, c.chip)
			chrome := g.planCovers(width, height)
			if c.stream {
				corner := ui.StreamArea(th, ui.StreamCorner, width, height, g.scene.TopChrome(), city.FitFooter)
				g.scene.SetCornerChrome(corner)
				chrome = append(chrome, corner)
			}
			require.NotEmpty(t, chrome, "precondition: some chrome lies over the plan")
			g.scene.SetPlanCovers(g.planCovers(width, height))
			plan := g.scene.CityPlan()
			overlapping := func(box city.Rect) []city.Rect {
				var out []city.Rect
				for _, r := range chrome {
					if r.Overlaps(box) {
						out = append(out, r)
					}
				}
				return out
			}

			t.Run("the compass should overlap no chrome", func(t *testing.T) {
				assert.Empty(t, overlapping(plan.Compass.Box))
			})

			t.Run("the cartouche should overlap no chrome", func(t *testing.T) {
				assert.Empty(t, overlapping(plan.Cartouche.Box))
			})

			t.Run("no badge should overlap any chrome", func(t *testing.T) {
				var over []string
				for _, b := range plan.Badges {
					if len(overlapping(b.Rect)) > 0 {
						over = append(over, b.Name)
					}
				}
				assert.Empty(t, over)
			})
		})
	}

	t.Run("when a badge in the middle of the plan is hovered", func(t *testing.T) {
		g := newGame(t, true)
		plan := g.scene.CityPlan()
		middle := plan.Area.Center()
		var hovered city.PlanBadge
		best := math.Inf(1)
		for _, b := range plan.Badges {
			if d := math.Hypot(b.At.X-middle.X, b.At.Y-middle.Y); d < best {
				hovered, best = b, d
			}
		}
		g.scene.SetHover(city.Hit{District: hovered.District})
		card, ok := g.scene.Card()
		require.True(t, ok, "precondition: the hover raises a card")
		laid := g.raisedCard(card, plan.Area)

		t.Run("its card should stand beside the badge", func(t *testing.T) {
			gap := math.Max(laid.Rect.Min.X-hovered.Rect.Max.X, hovered.Rect.Min.X-laid.Rect.Max.X)
			assert.InDelta(t, th.Grid(), gap, 1)
		})

		t.Run("its card should overlap neither the compass nor the cartouche", func(t *testing.T) {
			assert.Equal(t, []bool{false, false}, []bool{laid.Rect.Overlaps(plan.Compass.Box), laid.Rect.Overlaps(plan.Cartouche.Box)})
		})
	})

	t.Run("when the plan is open, the minimap should be hidden", func(t *testing.T) {
		assert.False(t, newGame(t, true).minimapAllowed())
	})
}
