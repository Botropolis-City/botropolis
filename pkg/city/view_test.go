package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

func viewed(t *testing.T, v city.View, sessions ...state.Session) *city.Scene {
	t.Helper()
	s := still(t, sessions...)
	s.SetView(v)
	return s
}

func buildingOf(t *testing.T, s *city.Scene, id string) *city.Building {
	t.Helper()
	for _, b := range s.City().Buildings() {
		if b.Session.ID == id {
			return b
		}
	}
	t.Fatalf("no building for %s", id)
	return nil
}

func TestViewScale(t *testing.T) {
	t.Run("when the city is drawn in Attention", func(t *testing.T) {
		t.Run("it should colour nothing, because the state tones already do", func(t *testing.T) {
			assert.Equal(t, city.ScaleNone, city.ViewAttention.Scale())
		})
	})

	t.Run("when a view counts something", func(t *testing.T) {
		t.Run("it should run along a ramp", func(t *testing.T) {
			for _, v := range []city.View{city.ViewSpend, city.ViewPressure, city.ViewStaleness, city.ViewTraffic, city.ViewFanout, city.ViewHealth} {
				assert.Equal(t, city.ScaleRamp, v.Scale(), v.Name())
			}
		})
	})

	t.Run("when a view sorts into kinds", func(t *testing.T) {
		t.Run("it should hand out categories", func(t *testing.T) {
			for _, v := range []city.View{city.ViewServers, city.ViewModels} {
				assert.Equal(t, city.ScaleCategory, v.Scale(), v.Name())
			}
		})
	})

	t.Run("when every view is asked its name", func(t *testing.T) {
		t.Run("it should have one, and no two the same", func(t *testing.T) {
			seen := map[string]bool{}
			for _, v := range city.Views {
				require.NotEmpty(t, v.Name())
				require.False(t, seen[v.Name()], v.Name())
				seen[v.Name()] = true
			}
			assert.Len(t, seen, 10)
		})
	})
}

func TestTint(t *testing.T) {
	t.Run("when one session has spent twice what another has", func(t *testing.T) {
		rich := session("rich", cinders, state.Working)
		poor := session("poor", botropolis, state.Working)
		s := viewed(t, city.ViewSpend, rich, poor)
		s.SetLedger(&fakeLedger{sessions: map[string]stats.Cost{"rich": cost(8), "poor": cost(4)}})

		t.Run("it should sit at the top of the ramp", func(t *testing.T) {
			assert.InDelta(t, 1.0, s.Tint(buildingOf(t, s, "rich")).Value, 1e-9)
		})

		t.Run("it should put the other half way", func(t *testing.T) {
			assert.InDelta(t, 0.5, s.Tint(buildingOf(t, s, "poor")).Value, 1e-9)
		})
	})

	t.Run("when a session's cost is not known", func(t *testing.T) {
		unknown := session("unknown", cinders, state.Working)
		s := viewed(t, city.ViewSpend, unknown)
		s.SetLedger(&fakeLedger{sessions: map[string]stats.Cost{}})

		t.Run("it should be left out of the colouring rather than painted as nothing", func(t *testing.T) {
			assert.False(t, s.Tint(buildingOf(t, s, "unknown")).Known)
		})
	})

	t.Run("when pressure is read", func(t *testing.T) {
		full := session("full", cinders, state.Working)
		full.ContextPercent = 50
		s := viewed(t, city.ViewPressure, full)

		t.Run("it should be a share of the window, not of the busiest session", func(t *testing.T) {
			assert.InDelta(t, 0.5, s.Tint(buildingOf(t, s, "full")).Value, 1e-9)
		})
	})

	t.Run("when a session has been quiet longer than the ramp goes", func(t *testing.T) {
		old := session("old", cinders, state.Waiting)
		old.LastActivity = now.Add(-10 * city.StalenessCap)
		s := viewed(t, city.ViewStaleness, old)

		t.Run("it should sit at the far end rather than run off it", func(t *testing.T) {
			assert.InDelta(t, 1.0, s.Tint(buildingOf(t, s, "old")).Value, 1e-9)
		})
	})

	t.Run("when two sessions run different models", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.Model = "claude-opus-5"
		b := session("b", botropolis, state.Working)
		b.Model = "claude-fable-5-1"
		s := viewed(t, city.ViewModels, a, b)

		t.Run("it should give them different categories", func(t *testing.T) {
			assert.NotEqual(t, s.Tint(buildingOf(t, s, "a")).Category, s.Tint(buildingOf(t, s, "b")).Category)
		})

		t.Run("it should name both in the legend, in the order the colours go out", func(t *testing.T) {
			assert.Equal(t, []string{"claude-fable-5-1", "claude-opus-5"}, s.Categories(city.ViewModels))
		})
	})

	t.Run("when a session calls one server more than another", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.MCPCalls = map[string]int{"atlassian": 2, "playwright": 9}
		s := viewed(t, city.ViewServers, a)

		t.Run("it should be coloured by the one it leans on", func(t *testing.T) {
			require.Equal(t, []string{"playwright"}, s.Categories(city.ViewServers))
			assert.True(t, s.Tint(buildingOf(t, s, "a")).Known)
		})
	})

	t.Run("when a session calls no servers at all", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.MCPCalls = nil
		s := viewed(t, city.ViewServers, a)

		t.Run("it should be left out rather than put in someone else's colour", func(t *testing.T) {
			assert.False(t, s.Tint(buildingOf(t, s, "a")).Known)
		})
	})

	t.Run("when the city is in Attention", func(t *testing.T) {
		s := viewed(t, city.ViewAttention, session("a", cinders, state.Working))

		t.Run("it should tint nothing", func(t *testing.T) {
			assert.False(t, s.Tint(buildingOf(t, s, "a")).Known)
		})
	})
}

func TestLegend(t *testing.T) {
	t.Run("when a ramp view is on", func(t *testing.T) {
		rich := session("rich", cinders, state.Working)
		s := viewed(t, city.ViewSpend, rich)
		s.SetLedger(&fakeLedger{sessions: map[string]stats.Cost{"rich": cost(12.5)}})

		t.Run("it should say what the far end is worth", func(t *testing.T) {
			assert.Equal(t, "$12.50", s.Legend().High)
		})

		t.Run("it should start where the ramp starts", func(t *testing.T) {
			assert.Equal(t, "0", s.Legend().Low)
		})

		t.Run("it should offer a bar rather than a list", func(t *testing.T) {
			assert.True(t, s.Legend().Ramp)
		})

		t.Run("it should name no categories", func(t *testing.T) {
			assert.Empty(t, s.Legend().Categories)
		})
	})

	t.Run("when a categorical view is on", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.Model = "claude-opus-5"
		s := viewed(t, city.ViewModels, a)

		t.Run("it should name the categories in the order the colours go out", func(t *testing.T) {
			assert.Equal(t, []string{"claude-opus-5"}, s.Legend().Categories)
		})

		t.Run("it should offer a list rather than a bar", func(t *testing.T) {
			assert.False(t, s.Legend().Ramp)
		})
	})

	t.Run("when any view is on", func(t *testing.T) {
		s := viewed(t, city.ViewSpend)

		t.Run("it should carry the view's name, so the legend says what it is reading", func(t *testing.T) {
			assert.Equal(t, "spend", s.Legend().Title)
		})
	})

	t.Run("when the view changes", func(t *testing.T) {
		rich := session("rich", cinders, state.Working)
		rich.CostUSD, rich.CostKnown = 12.5, true
		s := viewed(t, city.ViewSpend, rich)
		before := s.Legend()
		s.SetView(city.ViewPressure)

		t.Run("it should forget what the last one worked out", func(t *testing.T) {
			assert.NotEqual(t, before, s.Legend())
		})
	})

	t.Run("when a ramp view has nothing to measure", func(t *testing.T) {
		s := viewed(t, city.ViewSpend)

		t.Run("it should still answer rather than divide by nothing", func(t *testing.T) {
			assert.Equal(t, "nothing spent", s.Legend().Note)
		})
	})

	t.Run("when attention is on, which colours nothing", func(t *testing.T) {
		s := viewed(t, city.ViewAttention)

		t.Run("it should have nothing to explain", func(t *testing.T) {
			assert.False(t, s.Legend().Shown)
		})
	})
}

func TestStalenessUsesTheClock(t *testing.T) {
	quiet := session("quiet", cinders, state.Waiting)
	quiet.LastActivity = now.Add(-city.StalenessCap / 2)
	s := viewed(t, city.ViewStaleness, quiet)

	t.Run("when a session went quiet half the ramp ago", func(t *testing.T) {
		t.Run("it should sit half way along it", func(t *testing.T) {
			assert.InDelta(t, 0.5, s.Tint(buildingOf(t, s, "quiet")).Value, 0.01)
		})
	})

	_ = time.Second
}

func TestCategoryCap(t *testing.T) {
	t.Run("when the palette is asked how many kinds it can carry", func(t *testing.T) {
		t.Run("it should never be asked for more colours than it has", func(t *testing.T) {
			assert.LessOrEqual(t, city.MaxCategories, len(ui.Categorical))
		})
	})

	t.Run("when more models are running than the map can colour", func(t *testing.T) {
		// Five kinds, one session each, so the counts tie and the order is
		// alphabetical: the first three earn a colour and the rest do not.
		var sessions []state.Session
		for i, cwd := range []string{cinders, botropolis, "/tmp/c", "/tmp/d", "/tmp/e"} {
			sess := session(string(rune('a'+i)), cwd, state.Working)
			sess.Model = fmt.Sprintf("model-%d", i)
			sessions = append(sessions, sess)
		}
		s := viewed(t, city.ViewModels, sessions...)

		t.Run("it should colour the three it can and fold the rest into one", func(t *testing.T) {
			assert.Equal(t, []string{"model-0", "model-1", "model-2", city.OtherCategory}, s.Categories(city.ViewModels))
		})

		t.Run("it should put a folded session in the other category", func(t *testing.T) {
			assert.Equal(t, city.MaxCategories, s.Tint(buildingOf(t, s, "e")).Category)
		})

		t.Run("it should paint that category the neutral rather than a second-hand hue", func(t *testing.T) {
			assert.Equal(t, ui.Uncategorised, ui.Category(s.Tint(buildingOf(t, s, "e")).Category))
		})
	})
}

func TestViewByName(t *testing.T) {
	t.Run("when a view's own name is looked up", func(t *testing.T) {
		t.Run("it should give back the view that answers to it", func(t *testing.T) {
			for _, v := range city.Views {
				got, ok := city.ViewByName(v.Name())
				require.True(t, ok, v.Name())
				require.Equal(t, v, got, v.Name())
			}
		})
	})

	t.Run("when a name belongs to no view", func(t *testing.T) {
		t.Run("it should say so rather than guess", func(t *testing.T) {
			_, ok := city.ViewByName("weather")
			assert.False(t, ok)
		})
	})
}

func TestTheViewIsRemembered(t *testing.T) {
	t.Run("when a view is chosen", func(t *testing.T) {
		s := viewed(t, city.ViewSpend)

		t.Run("it should be written into the layout, so the next run opens where you left off", func(t *testing.T) {
			assert.Equal(t, "spend", s.Layout().View)
		})
	})

	t.Run("when a layout remembers a view", func(t *testing.T) {
		l := city.NewLayout()
		l.View = "pressure"
		s := city.NewScene(l)

		t.Run("it should start in it", func(t *testing.T) {
			assert.Equal(t, city.ViewPressure, s.View())
		})
	})

	t.Run("when a layout names a view this build no longer has", func(t *testing.T) {
		l := city.NewLayout()
		l.View = "weather"
		s := city.NewScene(l)

		t.Run("it should open on attention rather than refuse to start", func(t *testing.T) {
			assert.Equal(t, city.ViewAttention, s.View())
		})
	})
}
