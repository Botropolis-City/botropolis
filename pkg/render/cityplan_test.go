package render

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 106: the City Plan is drawn in place of the isometric map, one
// zoom step past Skyline.
func TestCityPlanDraw(t *testing.T) {
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := 0; i < 6; i++ {
		cwd := fmt.Sprintf("/p/d%d", i)
		sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at, SubagentsInFlight: 1})
	}
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return at }, time.UTC)
	s.SetProjection(city.Isometric)
	s.SetInstant(true)
	s.Resize(1600, 1000)
	s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
	s.Skyline()
	s.Wheel(s.Size().Scale(0.5), -1)
	require.True(t, s.CityPlanOpen(), "precondition: the plan is open")
	plan := s.CityPlan()

	t.Run("when the plan is open", func(t *testing.T) {
		hits := planHits(plan)

		t.Run("it should note a hit per badge, standing for its district", func(t *testing.T) {
			var want, got []*city.District
			for _, b := range plan.Badges {
				want = append(want, b.District)
			}
			for _, h := range hits {
				got = append(got, h.hit.District)
			}
			assert.Equal(t, want, got)
		})

		t.Run("it should draw no buildings, drones or movers", func(t *testing.T) {
			var movers []string
			for _, h := range hits {
				if h.hit.Building != nil || h.hit.Mover() {
					movers = append(movers, fmt.Sprint(h.hit))
				}
			}
			assert.Empty(t, movers)
		})
	})

	t.Run("the help should say how to reach the City Plan", func(t *testing.T) {
		var rows []string
		for _, k := range bindings {
			if strings.Contains(k.Action, "City Plan") {
				rows = append(rows, k.Key)
			}
		}
		assert.NotEmpty(t, rows)
	})
}

// Item 106: a compact badge keeps how many need you, and nothing else.
func TestCompactBadgeCount(t *testing.T) {
	for _, c := range []struct {
		name     string
		needsYou int
		want     string
	}{
		{"when two need you, it should carry the count", 2, "2"},
		{"when none need you, it should carry no count", 0, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, compactCount(city.PlanBadge{Name: "lanternfish", NeedsYou: c.needsYou, Sessions: 3, Subagents: 2, Compact: true}))
		})
	}
}

// Item 106: m opens and closes the City Plan from anywhere, Esc closes
// it, and the key can be rebound.
func TestCityPlanKey(t *testing.T) {
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	newGame := func(t *testing.T) *Game {
		t.Helper()
		var sessions []state.Session
		for i := 0; i < 4; i++ {
			cwd := fmt.Sprintf("/p/d%d", i)
			sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at})
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetProjection(city.Isometric)
		s.SetInstant(true)
		s.Resize(1200, 800)
		s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
		return &Game{scene: s, scripted: -1, clicked: -1}
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}

	t.Run("when m is pressed on the map", func(t *testing.T) {
		g := newGame(t)
		press(t, g, ebiten.KeyM)

		t.Run("it should open the plan", func(t *testing.T) {
			assert.True(t, g.scene.CityPlanOpen())
		})

		t.Run("and m is pressed again", func(t *testing.T) {
			press(t, g, ebiten.KeyM)

			t.Run("it should close it", func(t *testing.T) {
				assert.False(t, g.scene.CityPlanOpen())
			})
		})
	})

	t.Run("when escape is pressed on the plan", func(t *testing.T) {
		g := newGame(t)
		press(t, g, ebiten.KeyM)
		press(t, g, ebiten.KeyEscape)

		t.Run("it should close it", func(t *testing.T) {
			assert.False(t, g.scene.CityPlanOpen())
		})
	})

	t.Run("when the plan's key is rebound to j", func(t *testing.T) {
		chords := defaultChords()
		chords.Plan = config.Chord{Key: "j"}

		t.Run("and j is pressed", func(t *testing.T) {
			g := newGame(t)
			g.chords = &chords
			press(t, g, ebiten.KeyJ)

			t.Run("it should open the plan", func(t *testing.T) {
				assert.True(t, g.scene.CityPlanOpen())
			})
		})

		t.Run("and m is pressed", func(t *testing.T) {
			g := newGame(t)
			g.chords = &chords
			press(t, g, ebiten.KeyM)

			t.Run("it should leave the plan closed", func(t *testing.T) {
				assert.False(t, g.scene.CityPlanOpen())
			})
		})

		t.Run("the help should name j for the City Plan", func(t *testing.T) {
			g := newGame(t)
			g.chords = &chords
			var keys []string
			for _, k := range g.helpKeys() {
				if strings.Contains(k.Action, "City Plan") {
					keys = append(keys, k.Key)
				}
			}
			assert.Contains(t, keys, "j")
		})
	})

	t.Run("the help should list m for the City Plan", func(t *testing.T) {
		var keys []string
		for _, k := range newGame(t).helpKeys() {
			if strings.Contains(k.Action, "City Plan") {
				keys = append(keys, k.Key)
			}
		}
		assert.Contains(t, keys, "m")
	})

	t.Run("the view key's line should name f for Skyline and m for the City Plan", func(t *testing.T) {
		line := ui.ViewsNote("f", "m")
		assert.Equal(t, []bool{true, true}, []bool{strings.Contains(line, "f Skyline"), strings.Contains(line, "m the City Plan")})
	})
}
