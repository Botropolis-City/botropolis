package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Labels that claim things, held against the things they claim.
//
// The footer said "click attach" while a click attached, and went on
// saying it after a click stopped attaching. That is the one shape in
// this project that cannot be fixed by making one side derive from the
// other — a verb in a key row cannot be computed from what the code
// does without something silly — so it has to be tested instead.
//
// The map is full of labels of this kind now: the footer's verbs, the
// view key's nine questions, a legend's aggregate, five mover cards.
// Each is a promise that can quietly stop being true.
//
// These are the ones that can be checked cheaply. The gap, left open on
// purpose rather than forgotten: the five mover cards and the legend's
// aggregate are not tied to anything, and they are the likeliest to
// drift — a card's sentence is assembled from several fields, so any one
// of them can change meaning while the sentence reads the same. See
// rules/two-things-that-must-agree.md.

func action(keys []ui.Key, key string) (string, bool) {
	for _, k := range keys {
		if k.Key == key {
			return k.Action, true
		}
	}
	return "", false
}

func TestFooterVerbsMatchWhatTheyDo(t *testing.T) {
	scene := city.NewScene(city.NewLayout())
	scene.Resize(800, 600)
	scene.SetSnapshot(state.Snapshot{Sessions: []state.Session{
		{ID: "a", CWD: "/tmp/repo", State: state.Working},
	}})
	b := scene.City().Buildings()[0]

	t.Run("when the key rows name what a click does", func(t *testing.T) {
		for _, rows := range map[string][]ui.Key{"footer": footerKeys, "help": bindings} {
			verb, ok := action(rows, "click")
			require.True(t, ok)

			t.Run("it should say select, because clicking selects and does nothing else", func(t *testing.T) {
				require.Equal(t, "select", verb)
			})
		}
	})

	t.Run("when a building is actually clicked", func(t *testing.T) {
		toBuildings(scene)
		got := scene.Click(scene.Camera().WorldToScreen(b.Rect.Center()))

		t.Run("it should ask for nothing, which is what select means", func(t *testing.T) {
			assert.Equal(t, city.Action{}, got)
		})

		t.Run("it should have selected, which is the other half of the claim", func(t *testing.T) {
			assert.NotNil(t, scene.Selected())
		})
	})

	t.Run("when the key rows name what enter does", func(t *testing.T) {
		verb, ok := action(bindings, "enter")
		require.True(t, ok)

		t.Run("it should still promise an attach", func(t *testing.T) {
			require.Contains(t, verb, "attach")
		})

		t.Run("and activating the selection should be one", func(t *testing.T) {
			assert.Equal(t, city.ActionAttach, scene.Activate().Kind)
		})
	})

	t.Run("when the key rows name what v does", func(t *testing.T) {
		verb, ok := action(footerKeys, "v")
		require.True(t, ok)

		t.Run("it should say lenses, not something about cycling", func(t *testing.T) {
			assert.Equal(t, "lenses", verb)
		})
	})
}

// The view key promises nine questions. A view with no question is a row
// that says nothing, and a duplicate is two rows that say the same.
func TestEveryViewAnswersSomething(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range city.Views {
		t.Run("when "+v.Name()+" is listed", func(t *testing.T) {
			t.Run("it should have a question of its own", func(t *testing.T) {
				require.NotEmpty(t, v.Question())
				require.False(t, seen[v.Question()], v.Question())
				seen[v.Question()] = true
			})
		})
	}
}
