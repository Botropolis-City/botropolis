package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 108: while a hovered car fades everything not tied to it, what is
// drawn faded goes to a layer that is laid over the screen at the fade
// before the next thing kept at full strength, so the depth order holds.
func TestFader(t *testing.T) {
	real, layer := new(ebiten.Image), new(ebiten.Image)
	screen := real
	var log []string
	f := &fader{target: &screen, real: real, layer: layer, flush: func() { log = append(log, "flush") }}
	draw := func(name string) func() {
		return func() {
			where := "screen"
			if screen == layer {
				where = "layer"
			}
			log = append(log, name+" on the "+where)
		}
	}

	t.Run("when faded and kept things are drawn in depth order", func(t *testing.T) {
		for _, d := range []func(){f.faded(draw("a tree")), f.faded(draw("a site")), f.kept(draw("the car")), f.faded(draw("a lamp"))} {
			d()
		}
		f.done()

		t.Run("it should fade what is not kept and lay it down before each kept thing", func(t *testing.T) {
			assert.Equal(t, []string{"a tree on the layer", "a site on the layer", "flush", "the car on the screen", "a lamp on the layer", "flush"}, log)
		})

		t.Run("it should leave the screen as the target", func(t *testing.T) {
			assert.Same(t, real, screen)
		})
	})
}

func TestKeepsUnderACar(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c"} {
		sessions = append(sessions, state.Session{ID: name, CWD: "/p/" + name, State: state.Working, StartedAt: at})
	}
	// A brand-new session in c: its building is a site under construction.
	sessions = append(sessions, state.Session{ID: "new", CWD: "/p/c", State: state.Working, StartedAt: at})
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return at }, time.UTC)
	scene.Resize(1600, 1000)
	scene.SetSnapshot(state.Snapshot{At: at, Sessions: sessions, Roads: []state.Road{{From: "/p/a", To: "/p/b", Files: 1, Sessions: 1, Active: 1}}})
	c := scene.City()
	require.Len(t, c.Streets, 1, "precondition: one road")
	g := &Game{scene: scene}
	district := func(name string) *city.District {
		for _, d := range c.Districts {
			if d.Name == name {
				return d
			}
		}
		return nil
	}

	t.Run("when the pointer is on a car", func(t *testing.T) {
		scene.SetHover(city.Hit{Car: &city.CarHit{Road: c.Streets[0].Road}})
		g.lit = scene.Highlight()

		t.Run("it should keep the car's two districts and everything on them", func(t *testing.T) {
			assert.Equal(t, [2]bool{true, true}, [2]bool{g.keeps(district("a")), g.keeps(district("b"))})
		})

		t.Run("it should fade another district, its new site and all", func(t *testing.T) {
			assert.False(t, g.keeps(district("c")))
		})

		t.Run("it should keep the car", func(t *testing.T) {
			assert.True(t, g.keepsRoad(c.Streets[0].Road))
		})
	})

	t.Run("when the pointer is on nothing", func(t *testing.T) {
		scene.SetHover(city.Hit{})
		g.lit = scene.Highlight()

		t.Run("it should keep everything", func(t *testing.T) {
			assert.True(t, g.keeps(district("c")))
		})
	})
}
