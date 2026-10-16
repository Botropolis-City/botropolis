package city_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestBuildingGroup(t *testing.T) {
	grouped := func(joined string) state.Snapshot {
		a := session("a", cinders, state.NeedsYou)
		a.Group, a.GroupJoined = "g1", joined
		return state.Snapshot{
			Sessions: []state.Session{a, session("b", cinders, state.Working)},
			Groups: []state.Group{{ID: "g1", Name: "PROJ-123 · rate-limit headers", Short: "PROJ-123",
				Members: []string{"a"}, Move: state.MoveYours, Why: "a needs you"}},
		}
	}
	card := func(snapshot state.Snapshot, id string) city.Card {
		c := city.Build(snapshot, city.NewLayout())
		for _, b := range c.Buildings() {
			if b.Session.ID == id {
				return b.Card(now)
			}
		}
		t.Fatalf("no building %s", id)
		return city.Card{}
	}
	groupLines := func(c city.Card) []string {
		var lines []string
		for _, l := range c.Details {
			if strings.HasPrefix(l, "group ") || strings.HasPrefix(l, "joined ") {
				lines = append(lines, l)
			}
		}
		return lines
	}

	t.Run("when the session is in a group", func(t *testing.T) {
		lines := groupLines(card(grouped("mentions PROJ-123"), "a"))

		t.Run("it should name the group, whose move it is and why, then why the session joined", func(t *testing.T) {
			assert.Equal(t, []string{
				"group    PROJ-123 · rate-limit headers, yours: a needs you",
				"joined   mentions PROJ-123",
			}, lines)
		})
	})

	t.Run("when the session joined on a likely guess", func(t *testing.T) {
		lines := groupLines(card(grouped("likely: started with b"), "a"))

		t.Run("it should say the join is a guess", func(t *testing.T) {
			assert.Contains(t, lines, "joined   likely: started with b")
		})
	})

	t.Run("when a building is selected", func(t *testing.T) {
		selected := func(t *testing.T, snap state.Snapshot) *city.Scene {
			t.Helper()
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return now }, time.UTC)
			s.Resize(800, 600)
			s.SetSnapshot(snap)
			s.Click(centreOf(t, s, "a"))
			return s
		}

		t.Run("and there are groups", func(t *testing.T) {
			s := selected(t, grouped("mentions PROJ-123"))

			t.Run("it should offer to move it to a group", func(t *testing.T) {
				assert.Contains(t, s.Actions(), city.ActionMoveToGroup)
			})

			t.Run("and that is chosen", func(t *testing.T) {
				action, _ := s.Act(city.ActionMoveToGroup)

				t.Run("it should ask for the group to move that session to", func(t *testing.T) {
					assert.Equal(t, city.Action{Kind: city.ActionMoveToGroup, SessionID: "a"}, action)
				})
			})
		})

		t.Run("and there are none", func(t *testing.T) {
			snap := grouped("")
			snap.Groups = nil
			s := selected(t, snap)

			t.Run("it should not offer it", func(t *testing.T) {
				assert.NotContains(t, s.Actions(), city.ActionMoveToGroup)
			})
		})
	})

	t.Run("when the session is loose", func(t *testing.T) {
		lines := groupLines(card(grouped("mentions PROJ-123"), "b"))

		t.Run("it should carry no group lines", func(t *testing.T) {
			assert.Empty(t, lines)
		})
	})
}
