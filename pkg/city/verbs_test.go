package city_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 350: the card's face is one direct verb and three columns, and
// the columns hold the same verbs the flat row held, grouped.
func TestVerbs(t *testing.T) {
	selected := func(t *testing.T, id string, snap state.Snapshot) *city.Scene {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		s.Click(centreOf(t, s, id))
		require.NotNil(t, s.Selected())
		return s
	}
	grouped := func(id string, st state.State) state.Session {
		sess := session(id, cinders, st)
		sess.Group = "g1"
		return sess
	}
	group := state.Group{ID: "g1", Name: "Rate-limit headers", Members: []string{"a", "a2"}}
	// everything is every verb in every column, in one list.
	everything := func(v city.Verbs) []city.ActionKind {
		return slices.Concat(v.Session, v.Project, v.Group)
	}
	// flat is Actions() with the details fold taken out, which is what
	// the columns must hold between them.
	flat := func(s *city.Scene) []city.ActionKind {
		var kinds []city.ActionKind
		for _, k := range s.Actions() {
			if k != city.ActionDetails && k != city.ActionDetailsHide {
				kinds = append(kinds, k)
			}
		}
		return kinds
	}
	type fixture struct {
		name string
		open func(t *testing.T) *city.Scene
	}
	fixtures := []fixture{
		{"a live loose session", func(t *testing.T) *city.Scene {
			return selected(t, "a", snapshot(session("a", cinders, state.NeedsYou)))
		}},
		{"a live grouped session", func(t *testing.T) *city.Scene {
			return selected(t, "a", state.Snapshot{At: now, Sessions: []state.Session{grouped("a", state.NeedsYou), grouped("a2", state.Working)}, Groups: []state.Group{group}})
		}},
		{"a resting session", func(t *testing.T) *city.Scene {
			return selected(t, "a", snapshot(session("a", cinders, state.Resting)))
		}},
		{"a packed crate", func(t *testing.T) *city.Scene {
			crated := session("b", cinders, state.Resting)
			crated.Packed = true
			return selected(t, "b", snapshot(session("a", cinders, state.Working), crated))
		}},
		{"a session with a suggested group", func(t *testing.T) *city.Scene {
			loose := session("c", botropolis, state.Working)
			loose.Suggested, loose.SuggestedWhy = "g1", "started with Title a"
			return selected(t, "c", state.Snapshot{At: now, Sessions: []state.Session{grouped("a", state.Working), grouped("a2", state.Working), loose}, Groups: []state.Group{group}})
		}},
	}

	t.Run("when a live loose session is selected", func(t *testing.T) {
		v := fixtures[0].open(t).Verbs()

		t.Run("it should make attach the direct verb", func(t *testing.T) {
			assert.Equal(t, city.ActionAttach, v.Direct)
		})

		t.Run("it should fill the Session column in the columns' order", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionAttach, city.ActionStop, city.ActionRead, city.ActionGenerate, city.ActionPack, city.ActionDemolish}, v.Session)
		})

		t.Run("it should fill the Project column in the columns' order", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionNew, city.ActionReveal, city.ActionCopyPath, city.ActionStar, city.ActionHide, city.ActionDistrictType}, v.Project)
		})

		t.Run("it should hold move to group alone in the Group column", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionMoveToGroup}, v.Group)
		})
	})

	t.Run("when a live grouped session is selected", func(t *testing.T) {
		s := fixtures[1].open(t)

		t.Run("it should fill the Group column with the group's verbs", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionMoveToGroup, city.ActionTakeOut, city.ActionRenameGroup, city.ActionCloseGroup, city.ActionCopyStandup}, s.Verbs().Group)
		})

		t.Run("and its group joined on a word", func(t *testing.T) {
			worded := grouped("a", state.NeedsYou)
			worded.GroupWords = []string{"headers"}
			s := selected(t, "a", state.Snapshot{At: now, Sessions: []state.Session{worded, grouped("a2", state.Working)}, Groups: []state.Group{group}})

			t.Run("it should end the Group column with mute a word", func(t *testing.T) {
				assert.Equal(t, city.ActionMuteWord, s.Verbs().Group[len(s.Verbs().Group)-1])
			})
		})
	})

	t.Run("when a resting session is selected", func(t *testing.T) {
		v := fixtures[2].open(t).Verbs()

		t.Run("it should make resume the direct verb", func(t *testing.T) {
			assert.Equal(t, city.ActionResume, v.Direct)
		})

		t.Run("it should offer no stop", func(t *testing.T) {
			assert.NotContains(t, v.Session, city.ActionStop)
		})
	})

	t.Run("when a packed crate is selected", func(t *testing.T) {
		v := fixtures[3].open(t).Verbs()

		t.Run("it should make unpack the direct verb", func(t *testing.T) {
			assert.Equal(t, city.ActionUnpack, v.Direct)
		})

		t.Run("it should hold resume, unpack and demolish in the Session column", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionResume, city.ActionUnpack, city.ActionDemolish}, v.Session)
		})

		t.Run("it should leave the Project and Group columns empty", func(t *testing.T) {
			assert.Empty(t, slices.Concat(v.Project, v.Group))
		})
	})

	t.Run("when a group is suggested", func(t *testing.T) {
		v := fixtures[4].open(t).Verbs()

		t.Run("it should offer join suggested and move to group as two rows", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionMoveToGroup, city.ActionJoinSuggested}, v.Group)
		})
	})

	t.Run("when every pull request in its group is merged", func(t *testing.T) {
		a := grouped("a", state.Working)
		a.PRs = []claude.PR{{Number: 1, URL: "https://example.com/pr/1", State: claude.PRMerged}}
		s := selected(t, "a", state.Snapshot{At: now, Sessions: []state.Session{a, grouped("a2", state.Working)}, Groups: []state.Group{group}})

		t.Run("it should make close group the direct verb", func(t *testing.T) {
			assert.Equal(t, city.ActionCloseGroup, s.Verbs().Direct)
		})
	})

	t.Run("for every lens", func(t *testing.T) {
		firsts := map[city.Lens]city.ActionKind{
			city.LensAttention:   city.ActionAttach,
			city.LensWork:        city.ActionFocusLine,
			city.LensCost:        city.ActionBreakdown,
			city.LensCapacity:    city.ActionFork,
			city.LensHealth:      city.ActionOpenPR,
			city.LensConnections: city.ActionShowRoad,
		}
		for _, l := range city.Lenses {
			t.Run("when it is "+l.Name(), func(t *testing.T) {
				s := connected(t)
				s.SelectLens(l)

				t.Run("it should make the lens's first verb the direct one", func(t *testing.T) {
					assert.Equal(t, firsts[l], s.Verbs().Direct)
				})
			})
		}
	})

	t.Run("for every fixture state", func(t *testing.T) {
		for _, f := range fixtures {
			t.Run("when "+f.name+" is selected", func(t *testing.T) {
				s := f.open(t)
				v := s.Verbs()

				t.Run("the columns should hold what the flat row held, details aside, each once", func(t *testing.T) {
					want := flat(s)
					got := everything(v)
					slices.Sort(want)
					slices.Sort(got)
					assert.Equal(t, want, got)
				})

				t.Run("the direct verb should be the flat row's first", func(t *testing.T) {
					assert.Equal(t, flat(s)[0], v.Direct)
				})
			})
		}
	})

	t.Run("when the selection's card is read", func(t *testing.T) {
		s := fixtures[1].open(t)
		card, _, ok := s.SelectedCard()
		require.True(t, ok)

		t.Run("it should carry four buttons, the direct verb then the three columns", func(t *testing.T) {
			assert.Equal(t, []string{"attach", "Session ⌄", "Project ⌄", "Group ⌄"}, card.Actions)
		})

		t.Run("it should hint the direct verb with its description", func(t *testing.T) {
			require.Len(t, card.Hints, 5)
			assert.Equal(t, city.ActionAttach.Describe(), card.Hints[0])
		})

		t.Run("it should hint each column with its rows", func(t *testing.T) {
			assert.Equal(t, "Group: move to group, take out of group, rename group, close group, copy stand-up", card.Hints[3])
		})

		t.Run("it should hint the numbers row with the details fold", func(t *testing.T) {
			assert.Equal(t, city.ActionDetails.Describe(), card.Hints[4])
		})
	})

	t.Run("when a teammate in an agent team is selected", func(t *testing.T) {
		lead := session("lead", cinders, state.Working)
		mate := session("mate", botropolis, state.Working)
		mate.Lead = "lead"
		s := selected(t, "mate", snapshot(lead, mate))

		t.Run("it should describe move to group as moving its team", func(t *testing.T) {
			assert.Contains(t, s.DescribeVerb(city.ActionMoveToGroup), "and its team")
		})

		t.Run("it should describe other verbs as for any session", func(t *testing.T) {
			assert.Equal(t, city.ActionStop.Describe(), s.DescribeVerb(city.ActionStop))
		})
	})

	t.Run("when the columns are read as a list", func(t *testing.T) {
		t.Run("it should name three, Session, Project and Group", func(t *testing.T) {
			var titles []string
			for _, c := range city.VerbColumns {
				titles = append(titles, c.Title)
			}
			assert.Equal(t, []string{"Session", "Project", "Group"}, titles)
		})

		t.Run("it should list each kind in one column only", func(t *testing.T) {
			seen := map[city.ActionKind]int{}
			for _, c := range city.VerbColumns {
				for _, k := range c.Kinds {
					seen[k]++
				}
			}
			for k, n := range seen {
				assert.Equal(t, 1, n, string(k))
			}
		})
	})
}
