package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
)

// Enter typed straight after the words, before the typing pause has let
// the search run, waits for the search and opens its top result.
// Items 82 and 91.
func TestFinderEnterBeforeTheAnswer(t *testing.T) {
	t.Run("when Enter comes while the search for what was typed is still out", func(t *testing.T) {
		actor := &recorder{}
		scene := city.NewScene(city.NewLayout())
		scene.Resize(800, 600)
		scene.SetSnapshot(state.Snapshot{Sessions: []state.Session{{ID: "stopped", CWD: "/src/kiln", State: state.Resting}}})
		var out []func()
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1,
			search:   func(find.Query) ([]find.Hit, error) { return []find.Hit{{SessionID: "stopped", Dir: "/src/kiln"}}, nil },
			dispatch: func(run func()) { out = append(out, run) },
		}
		pressKey(t, g, ebiten.KeySlash, false)
		f, ok := g.overlays.top().(*finder)
		require.True(t, ok, "the finder should be open")
		f.now = func() time.Time { return time.Time{} }
		for _, r := range "kiln" {
			g.scriptedRune = r
			require.NoError(t, g.handleKeys())
		}
		g.scriptedRune = 0
		require.Empty(t, out, "the pause has not passed, so nothing should have been asked yet")
		pressKey(t, g, ebiten.KeyEnter, false)

		t.Run("it should ask at once", func(t *testing.T) {
			assert.Len(t, out, 1)
		})

		t.Run("it should not open anything yet", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})

		t.Run("and the answer arrives on a later tick", func(t *testing.T) {
			for _, run := range out {
				run()
			}
			require.NoError(t, g.handleKeys())
			g.settle()

			t.Run("it should open its top result", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionResume, SessionID: "stopped"}}, actor.done)
			})

			t.Run("it should close the finder", func(t *testing.T) {
				assert.False(t, g.finding())
			})
		})
	})
}
