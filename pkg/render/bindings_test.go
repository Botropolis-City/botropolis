package render

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
)

// Item 350, slice 5a: every map binding in config.Bindings is what the
// map answers. Each is rebound to k, a key nothing at the map uses, and
// pressing k must change what the city shows, while pressing its old
// default must change nothing.
func TestEveryMapBindingAnswers(t *testing.T) {
	// held is answered while it is held, not on a press, and has its own
	// case below.
	held := map[string]bool{"explain": true}
	rebound := func(t *testing.T, name, to string) *buttonPanRig {
		t.Helper()
		r := newButtonPanRig(t)
		r.g.probe = func(context.Context) {}
		keys := map[string]string{}
		if to != "" {
			keys[name] = to
		}
		chords, err := config.FindChords(config.Config{Keys: keys}, runtime.GOOS, KnownKey)
		require.NoError(t, err)
		r.g.chords = &chords
		// The City Plan opens only in the isometric projection.
		r.g.scene.SetProjection(city.Isometric)
		// Start in a lens of its own, so a lens key has somewhere to go.
		lens := city.LensHealth
		if name == "lens_5" {
			lens = city.LensCost
		}
		r.g.selectLens(lens)
		r.g.scene.Pan(city.Point{X: 40, Y: 40})
		return r
	}
	pressName := func(t *testing.T, g *Game, name string) {
		t.Helper()
		g.pressScripted(name)
		defer func() { g.scripted, g.scriptedShift = -1, false }()
		require.NoError(t, g.handleKeys())
		g.settle()
	}

	for _, b := range config.Bindings {
		if b.Scope != config.ScopeMap || held[b.Name] {
			continue
		}
		t.Run("when "+b.Name+" is rebound to k", func(t *testing.T) {
			t.Run("and k is pressed", func(t *testing.T) {
				// What the default does, unbound, is what k must do bound.
				d := rebound(t, b.Name, "")
				pressName(t, d.g, scriptName(b.Default))
				wanted := fingerprint(d)
				r := rebound(t, b.Name, "k")
				before := fingerprint(r)
				pressName(t, r.g, "k")

				t.Run("it should change what the city shows", func(t *testing.T) {
					assert.NotEqual(t, before, fingerprint(r))
				})

				t.Run("it should do what "+b.Default+" did", func(t *testing.T) {
					assert.Equal(t, wanted, fingerprint(r))
				})
			})

			t.Run("and its old key is pressed", func(t *testing.T) {
				r := rebound(t, b.Name, "k")
				before := fingerprint(r)
				pressName(t, r.g, scriptName(b.Default))

				t.Run("it should do nothing", func(t *testing.T) {
					assert.Equal(t, before, fingerprint(r))
				})
			})
		})
	}

	t.Run("when explain is rebound to k, and k is held", func(t *testing.T) {
		r := rebound(t, "explain", "k")
		r.g.scriptedHeld = map[ebiten.Key]bool{ebiten.KeyK: true}

		t.Run("it should explain", func(t *testing.T) {
			assert.True(t, r.g.explainHeld())
		})
	})
}

// Item 350, slice 5a: the reference names every map binding, by the key
// it is bound to.
func TestReferenceNamesEveryBinding(t *testing.T) {
	g := &Game{}
	text := ""
	for _, k := range g.helpKeys() {
		text += k.Key + "\n"
	}
	for _, b := range config.Bindings {
		if b.Scope != config.ScopeMap && b.Scope != config.ScopeEverywhere && b.Scope != config.ScopeFinder {
			continue
		}
		t.Run("when "+b.Name+" is read in the reference", func(t *testing.T) {
			t.Run("it should be listed under its key", func(t *testing.T) {
				assert.True(t, referenceLists(g, b.Name), "no row of the reference reads from %s", b.Name)
			})
		})
	}

	t.Run("when a binding is rebound", func(t *testing.T) {
		chords, err := config.FindChords(config.Config{Keys: map[string]string{"timeline": "k"}}, runtime.GOOS, KnownKey)
		require.NoError(t, err)
		g := &Game{chords: &chords}
		var keys []string
		for _, k := range g.helpKeys() {
			if strings.HasPrefix(k.Action, "timeline") {
				keys = append(keys, k.Key)
			}
		}

		t.Run("it should show the key it is bound to", func(t *testing.T) {
			assert.Equal(t, []string{"k"}, keys)
		})
	})
}

// scriptName is a default chord as --keys names it.
func scriptName(chord string) string {
	if chord == "shift+slash" {
		return "?"
	}
	return chord
}

// fingerprint is what the city shows that a key at the map can change.
func fingerprint(r *buttonPanRig) string {
	g := r.g
	var overlays []string
	for _, o := range g.overlays {
		overlays = append(overlays, fmt.Sprintf("%T", o))
	}
	cam := g.scene.Camera()
	return fmt.Sprint(overlays, g.settingsOpen, g.breakdown, g.timeline, g.viewKey, g.hidden, g.sidebar, g.snap != "",
		g.scene.Lens(), g.scene.View(), g.scene.CityPlanOpen(), g.scene.DetailsOpen(), g.scene.Labels(), g.streamShown(),
		g.scene.Light(), g.scene.Clock().Round(1e9), cam.Offset, cam.Zoom, cam.Heading, g.finding(), g.share != nil,
		g.notice.Text(timeNow()), len(r.actor.done), g.scene.Selected() != nil)
}

// configured is the chords a config sets, read as the city reads them,
// so a test rebinds a key the way a person does.
func configured(cfg config.Config) *config.Chords {
	chords, err := config.FindChords(cfg, runtime.GOOS, KnownKey)
	if err != nil {
		panic(err)
	}
	return &chords
}
