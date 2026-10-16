package city_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestBuildingCardFace(t *testing.T) {
	hovered := func(t *testing.T, sessions ...state.Session) (*city.Scene, city.Card) {
		t.Helper()
		s := scene(t, sessions...)
		s.PointerMove(centreOf(t, s, sessions[0].ID))
		card, ok := s.Card()
		require.True(t, ok)
		return s, card
	}
	asking := session("a", cinders, state.NeedsYou)
	asking.Note = "Claude needs your permission to use Bash"
	asking.Recap = claude.Recap{Text: "We shipped the button.", At: now.Add(-time.Hour)}
	recapped := session("a", cinders, state.Waiting)
	recapped.Recap = claude.Recap{Text: "We shipped the button. Next: the changelog.", At: now.Add(-time.Hour)}
	prompted := session("a", cinders, state.Waiting)
	prompted.LastPrompt = "add a summary button"
	silent := session("a", cinders, state.Waiting)

	t.Run("when a session needs you, and it is asking a question", func(t *testing.T) {
		_, card := hovered(t, asking)

		t.Run("it should say what it is", func(t *testing.T) {
			assert.Equal(t, "session in cinders", card.Kind)
		})

		t.Run("it should say it is your move", func(t *testing.T) {
			assert.Equal(t, "your move", card.Move)
		})

		t.Run("it should lead with the question", func(t *testing.T) {
			assert.Equal(t, "Claude needs your permission to use Bash", card.Sentence)
		})
	})

	t.Run("when it has a recap and asks nothing", func(t *testing.T) {
		_, card := hovered(t, recapped)

		t.Run("it should lead with the recap", func(t *testing.T) {
			assert.Equal(t, "We shipped the button. Next: the changelog.", card.Sentence)
		})
	})

	t.Run("when it has no recap, and a summary was generated", func(t *testing.T) {
		s, _ := hovered(t, prompted)
		s.SetGenerated("a", city.Generated{Text: "A generated summary.", At: now.Add(-5 * time.Minute)})
		card, _ := s.Card()

		t.Run("it should lead with the generated summary", func(t *testing.T) {
			assert.Equal(t, "A generated summary.", card.Sentence)
		})
	})

	t.Run("when nothing has been said", func(t *testing.T) {
		t.Run("and there is a last prompt", func(t *testing.T) {
			_, card := hovered(t, prompted)

			t.Run("it should lead with what you asked", func(t *testing.T) {
				assert.Equal(t, "You asked: “add a summary button”", card.Sentence)
			})
		})

		t.Run("and there is none", func(t *testing.T) {
			_, card := hovered(t, silent)

			t.Run("it should have no sentence", func(t *testing.T) {
				assert.Empty(t, card.Sentence)
			})
		})
	})

	t.Run("when it is working", func(t *testing.T) {
		_, card := hovered(t, session("a", cinders, state.Working))

		t.Run("it should say it is the agents' move", func(t *testing.T) {
			assert.Equal(t, "the agents' move", card.Move)
		})
	})

	t.Run("when it is in a group", func(t *testing.T) {
		a, b, c, d := session("a", cinders, state.Working), session("b", botropolis, state.Working), session("c", mullet, state.Working), session("d", cinders, state.Working)
		a.StartedAt, b.StartedAt, c.StartedAt, d.StartedAt = now.Add(-4*time.Hour), now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour)
		a.Group, b.Group, c.Group, d.Group = "g1", "g1", "g1", "g1"
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{c, d, b, a},
			Groups: []state.Group{{ID: "g1", Name: "Rate-limit headers", Members: []string{"c", "d", "b", "a"}}}})
		s.PointerMove(centreOf(t, s, "b"))
		card, _ := s.Card()

		t.Run("it should name the group and its station, one per project in the order the group reached it", func(t *testing.T) {
			assert.Equal(t, "Rate-limit headers — station 2 of 3", card.Group)
		})
	})

	t.Run("when it is in no group", func(t *testing.T) {
		_, card := hovered(t, recapped)

		t.Run("it should have no group line", func(t *testing.T) {
			assert.Empty(t, card.Group)
		})
	})

	t.Run("for every view", func(t *testing.T) {
		leads := map[city.View]string{
			city.ViewAttention: "context",
			city.ViewSpend:     "spent",
			city.ViewPressure:  "context",
			city.ViewStaleness: "idle",
			city.ViewServers:   "server calls",
			city.ViewTraffic:   "crossings",
			city.ViewModels:    "model",
			city.ViewFanout:    "in flight",
			city.ViewHealth:    "errors",
			city.ViewGroups:    "station",
		}
		for _, v := range city.Views {
			t.Run("when it is "+v.Name(), func(t *testing.T) {
				s, _ := hovered(t, recapped)
				s.SetView(v)
				card, _ := s.Card()

				t.Run("it should carry at most three numbers", func(t *testing.T) {
					assert.LessOrEqual(t, len(card.Numbers), 3)
				})

				t.Run("it should lead with that view's number", func(t *testing.T) {
					require.NotEmpty(t, card.Numbers)
					assert.Equal(t, leads[v], card.Numbers[0].Label)
				})
			})
		}
	})

	t.Run("when its card is drawn", func(t *testing.T) {
		_, card := hovered(t, recapped)

		t.Run("it should fold today's table into details", func(t *testing.T) {
			assert.Contains(t, card.Details, "branch   main")
		})

		t.Run("it should keep no key-value lines on its face", func(t *testing.T) {
			assert.Empty(t, card.Lines)
		})
	})
}

func TestDistrictCardFace(t *testing.T) {
	c := build(t, city.NewLayout(), session("a", cinders, state.NeedsYou), session("b", cinders, state.Working))
	card := c.Districts[0].Card()

	t.Run("it should say what it is, whose move, and why", func(t *testing.T) {
		assert.Equal(t, []string{"project", "your move", "Fix the CI queue needs you"},
			[]string{card.Kind, card.Move, card.Sentence})
	})

	t.Run("it should fold its table into details", func(t *testing.T) {
		assert.True(t, strings.HasPrefix(card.Details[0], "sessions 2") && len(card.Lines) == 0)
	})
}
