package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// The five things on the map that move, and the one question each has to
// answer: what am I. They were the only objects with no card, which is
// why three of them — all vehicles — read as the same thing.
func TestMoverCards(t *testing.T) {
	t.Run("when a drone over a roof is pointed at", func(t *testing.T) {
		s := session("a", cinders, state.Working)
		s.SubagentsInFlight = 3
		sc := still(t, s)
		card := city.SubagentCard(buildingOf(t, sc, "a"))

		t.Run("it should say it is a subagent", func(t *testing.T) {
			assert.Contains(t, card.Title, "subagent")
		})

		t.Run("it should say how many are up", func(t *testing.T) {
			assert.Contains(t, card.Lines[0], "3")
		})
	})

	t.Run("when a car on a street is pointed at", func(t *testing.T) {
		road := city.RoadLine{Messages: 12, Files: 5}
		card := city.CarCard(&road, "cinders", "botropolis")

		t.Run("it should name both repos, which is the thing it could not say", func(t *testing.T) {
			assert.Contains(t, card.Title, "cinders")
			assert.Contains(t, card.Title, "botropolis")
		})

		t.Run("it should give the traffic it stands for", func(t *testing.T) {
			assert.Contains(t, card.Lines[0], "12")
		})

		t.Run("it should count the files separately from the messages", func(t *testing.T) {
			assert.Contains(t, card.Lines[1], "5")
		})
	})

	t.Run("when a flag on a roof is pointed at", func(t *testing.T) {
		card := city.FlagCard(claude.PR{Number: 617, Repository: "auroq/botropolis", State: claude.PRMerged})

		t.Run("it should say it is a pull request", func(t *testing.T) {
			assert.Contains(t, card.Title, "617")
		})

		t.Run("it should say which way it went", func(t *testing.T) {
			assert.Contains(t, card.Lines[0], "merged")
		})
	})

	t.Run("when a plume of smoke is pointed at", func(t *testing.T) {
		s := session("a", cinders, state.Working)
		s.APIErrors = 4
		sc := still(t, s)
		card := city.SmokeCard(buildingOf(t, sc, "a"))

		t.Run("it should say what is burning", func(t *testing.T) {
			assert.Contains(t, card.Title, "api errors")
		})

		t.Run("it should count them", func(t *testing.T) {
			assert.Contains(t, card.Lines[0], "4")
		})
	})

	t.Run("when every mover's card is built", func(t *testing.T) {
		s := session("a", cinders, state.Working)
		sc := still(t, s)
		b := buildingOf(t, sc, "a")
		road := city.RoadLine{}

		t.Run("it should name the session it belongs to, so the card is not orphaned", func(t *testing.T) {
			for _, c := range []city.Card{
				city.SubagentCard(b), city.SmokeCard(b),
			} {
				require.NotEmpty(t, c.Title)
				require.Contains(t, c.Title, b.Card(sc.City().Time).Title)
			}
			assert.NotEmpty(t, city.CarCard(&road, "a", "b").Title)
		})
	})

	// Item 89: the inspector and the delivery truck each mean one datum.
	t.Run("when an inspector's car is pointed at", func(t *testing.T) {
		s := session("a", cinders, state.Working)
		s.PRs = []claude.PR{{Number: 7, State: claude.PROpen}, {Number: 8, State: claude.PRMerged}, {Number: 9, State: claude.PROpen}}
		card := city.InspectorCard(buildingOf(t, still(t, s), "a"))

		t.Run("it should name the open PRs it is waiting on, and only those", func(t *testing.T) {
			assert.Equal(t, "waiting on #7, #9", card.Lines[0])
		})
	})

	t.Run("when a van is pointed at", func(t *testing.T) {
		card := city.MessageCard(city.Message{From: &city.District{Name: "cinders"}, To: &city.District{Name: "mullet"}})

		t.Run("it should say it is one message, from where to where", func(t *testing.T) {
			assert.Equal(t, "message -> cinders → mullet", card.Title)
		})
	})

	t.Run("when a delivery truck is pointed at", func(t *testing.T) {
		card := city.DeliveryCard(city.Delivery{PR: claude.PR{Number: 7, Repository: "o/r", State: claude.PRMerged}})

		t.Run("it should name the PR it carries", func(t *testing.T) {
			assert.Contains(t, card.Title, "#7 o/r")
		})
	})
}

func TestMoverHitsBeatTheBuildingUnderThem(t *testing.T) {
	// A drone circles a roof and a flag stands on one, so the building's
	// own hit rect covers both. Pointing at the drone has to answer about
	// the drone, or the card is the thing you were not asking about.
	s := session("a", cinders, state.Working)
	s.SubagentsInFlight = 2
	sc := still(t, s)
	b := buildingOf(t, sc, "a")

	for name, hit := range map[string]city.Hit{
		"a drone over the roof": {Building: b, Subagent: b},
		"a plume of smoke":      {Building: b, Smoke: b},
	} {
		t.Run("when "+name+" is pointed at", func(t *testing.T) {
			sc.SetHover(hit)
			card, ok := sc.Card()
			require.True(t, ok)

			t.Run("it should answer about the mover, not the building it sits on", func(t *testing.T) {
				assert.NotEqual(t, b.Card(sc.City().Time).Title, card.Title)
			})
		})
	}

	t.Run("when a car is pointed at", func(t *testing.T) {
		road := city.RoadLine{Messages: 3}
		sc.SetHover(city.Hit{Car: &city.CarHit{Road: &road, From: "cinders", To: "botropolis"}})
		card, ok := sc.Card()
		require.True(t, ok)

		t.Run("it should name the pair it runs between", func(t *testing.T) {
			assert.Contains(t, card.Title, "botropolis")
		})
	})

	t.Run("when a flag is pointed at", func(t *testing.T) {
		sc.SetHover(city.Hit{Building: b, Flag: &city.FlagHit{Building: b, PR: claude.PR{Number: 9}}})
		card, ok := sc.Card()
		require.True(t, ok)

		t.Run("it should answer about the pull request", func(t *testing.T) {
			assert.Contains(t, card.Title, "#9")
		})
	})
}
