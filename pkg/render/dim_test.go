package render

import (
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// lotPaving is a lot's paving as the fitted city draws it by day,
// sampled from a frame (item 359): the palest ground a dimmed building
// stands on, so the hardest to read a dim against.
var lotPaving = color.NRGBA{194, 192, 184, 0xff}

func luminance(c [3]float64) float64 { return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2] }

// laidOver lays an opaque sprite pixel, through a tint, over the ground,
// as Ebitengine does: the scale applies to the premultiplied source.
func laidOver(src, ground color.NRGBA, r, g, b, a float32) [3]float64 {
	s := [3]float64{float64(src.R) / 255 * float64(r), float64(src.G) / 255 * float64(g), float64(src.B) / 255 * float64(b)}
	d := [3]float64{float64(ground.R) / 255, float64(ground.G) / 255, float64(ground.B) / 255}
	var out [3]float64
	for i := range out {
		out[i] = s[i] + d[i]*(1-float64(a))
	}
	return out
}

// Item 359, slice 2: the dim dims, rather than ghosting a pale building
// over a pale lot.
func TestDim(t *testing.T) {
	tint := dimmed()
	r, g, b, a := tint.R(), tint.G(), tint.B(), tint.A()

	t.Run("when a white building pixel is laid through the dim over its lot", func(t *testing.T) {
		white := color.NRGBA{0xff, 0xff, 0xff, 0xff}
		out := luminance(laidOver(white, lotPaving, r, g, b, a))
		lot := luminance(laidOver(lotPaving, lotPaving, 1, 1, 1, 1))

		t.Run("it should come out darker than the lot, so it reads as dimmed and not as pale", func(t *testing.T) {
			assert.Less(t, out, lot)
		})
	})

	t.Run("it should keep at most a third of a pixel's own light", func(t *testing.T) {
		assert.LessOrEqual(t, r, float32(1)/3)
	})

	t.Run("it should add no hue of its own", func(t *testing.T) {
		assert.Equal(t, []float32{r, r}, []float32{g, b})
	})
}

// Every highlight dims what is outside its answer with the one dim.
func TestUnlitUnderEveryHighlight(t *testing.T) {
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	mk := func(id, cwd, group string, calls map[string]int) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.Working, Group: group, StartedAt: at, LastActivity: at.Add(-time.Minute), MCPCalls: calls}
	}
	sessions := []state.Session{
		mk("a", "/p/a", "g1", map[string]int{"files": 2}),
		mk("a2", "/p/b", "g1", map[string]int{"search": 1}),
		mk("out", "/p/c", "", map[string]int{"search": 3}),
	}
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return at }, time.UTC)
	scene.Resize(1600, 1000)
	scene.SetSnapshot(state.Snapshot{At: at, Sessions: sessions,
		Groups:  []state.Group{{ID: "g1", Members: []string{"a", "a2"}}},
		Servers: state.Servers(sessions, claude.MCPConfig{})})
	c := scene.City()
	building := func(id string) *city.Building {
		for _, b := range c.Buildings() {
			if b.Session.ID == id {
				return b
			}
		}
		t.Fatalf("no building %s", id)
		return nil
	}
	var files *city.Tower
	for _, tw := range c.Towers {
		if tw.Server.Name == "files" {
			files = tw
		}
	}
	require.NotNil(t, files)

	for _, hl := range []struct {
		name    string
		set     func()
		inside  string
		outside string
	}{
		{"a tower is pointed at", func() { scene.SetHover(city.Hit{Tower: files}) }, "a", "out"},
		{"a building in no group is pointed at", func() { scene.SetHover(city.Hit{Building: building("out")}) }, "out", "a"},
		{"a grouped building is pointed at", func() { scene.SetHover(city.Hit{Building: building("a")}) }, "a2", "out"},
		{"a group is focused", func() { require.True(t, scene.FocusGroup("g1")) }, "a2", "out"},
	} {
		t.Run("when "+hl.name, func(t *testing.T) {
			hl.set()
			t.Cleanup(func() { scene.SetHover(city.Hit{}); scene.ClearFocus() })
			g := &Game{scene: scene, theme: ui.NewTheme(1)}
			g.lit = scene.Highlight()

			t.Run("it should dim a building outside the answer", func(t *testing.T) {
				assert.True(t, g.unlit(building(hl.outside)))
			})

			t.Run("it should leave a building inside the answer undimmed", func(t *testing.T) {
				assert.False(t, g.unlit(building(hl.inside)))
			})
		})
	}
}
