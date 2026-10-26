package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 273: the scripted runs row takes the map live, and the finder
// leaves out what the map leaves out.
func TestScriptedRunsSetting(t *testing.T) {
	for _, c := range []struct {
		value string
		want  bool
	}{{config.ScriptedShown, true}, {config.ScriptedHidden, false}} {
		t.Run("when the panel sets scripted runs "+c.value, func(t *testing.T) {
			g := &Game{scene: city.NewScene(city.NewLayout())}
			g.scene.SetShown(city.Shown{Scripted: !c.want})
			g.applySetting(ui.Setting{Key: config.KeyScriptedRuns, Label: "scripted runs", Value: c.value})

			t.Run("it should set the scene to match", func(t *testing.T) {
				assert.Equal(t, c.want, g.scene.Shown().Scripted)
			})
		})
	}
}

func TestFinderLeavesOutHiddenScriptedRuns(t *testing.T) {
	hits := []find.Hit{{SessionID: "run", Dir: "/src/pipeline", Scripted: true}, {SessionID: "person", Dir: "/src/kiln"}}
	found := func(t *testing.T, shown city.Shown) []string {
		t.Helper()
		scene := city.NewScene(city.NewLayout())
		scene.Resize(800, 600)
		scene.SetShown(shown)
		var out []func()
		g := &Game{scene: scene, actor: &recorder{}, scripted: -1, clicked: -1,
			search:   func(find.Query) ([]find.Hit, error) { return hits, nil },
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
		f.follow(g, true)
		for _, run := range out {
			run()
		}
		var ids []string
		for _, h := range f.results {
			ids = append(ids, h.SessionID)
		}
		return ids
	}

	t.Run("when scripted runs are hidden", func(t *testing.T) {
		t.Run("it should list only the interactive session", func(t *testing.T) {
			assert.Equal(t, []string{"person"}, found(t, city.Shown{}))
		})
	})

	t.Run("when scripted runs are shown", func(t *testing.T) {
		t.Run("it should list both", func(t *testing.T) {
			assert.Equal(t, []string{"run", "person"}, found(t, city.Shown{Scripted: true}))
		})
	})
}
