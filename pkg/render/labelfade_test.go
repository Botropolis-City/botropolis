package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 102: pointing at a car fades every label outside what the car
// ties together, as the world fades; the labels of its two districts
// and its road's tag stay full.
func TestLabelsFadeWithTheHighlight(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c", "d"} {
		sessions = append(sessions, state.Session{ID: name, CWD: "/p/" + name, State: state.Working, StartedAt: at})
	}
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return at }, time.UTC)
	scene.Resize(1600, 1000)
	scene.SetSnapshot(state.Snapshot{At: at, Sessions: sessions, Roads: []state.Road{
		{From: "/p/a", To: "/p/b", Files: 1, Sessions: 1, Active: 1},
		{From: "/p/c", To: "/p/d", Files: 1, Sessions: 1, Active: 1},
	}})
	c := scene.City()
	require.Len(t, c.Streets, 2, "precondition: two roads")
	building := map[string]*city.Building{}
	for _, b := range c.Buildings() {
		b.Group = &state.Group{Short: "g"}
		building[b.Session.ID] = b
	}
	th := ui.NewTheme(1)
	faces, err := newFaces(th)
	require.NoError(t, err)
	g := &Game{scene: scene, theme: th, faces: faces}
	// faded queues the label and reports whether it is to be drawn faded.
	faded := func(queue func()) bool {
		g.mapLabels = g.mapLabels[:0]
		queue()
		require.Len(t, g.mapLabels, 1)
		return g.mapLabels[0].faded
	}
	badge := func(id string) func() {
		return func() { g.queueBadge(building[id], city.Point{X: 100, Y: 100}) }
	}
	tag := func(i int) func() {
		return func() { g.queueRoadTag(c.Streets[i].Road, city.Point{X: 100, Y: 100}) }
	}

	t.Run("when the pointer is on a car", func(t *testing.T) {
		scene.SetHover(city.Hit{Car: &city.CarHit{Road: c.Streets[0].Road}})
		g.lit = scene.Highlight()

		t.Run("it should fade a badge in a district off its road", func(t *testing.T) {
			assert.True(t, faded(badge("c")))
		})

		t.Run("it should not fade a badge in either of its districts", func(t *testing.T) {
			assert.Equal(t, []bool{false, false}, []bool{faded(badge("a")), faded(badge("b"))})
		})

		t.Run("it should not fade its road's tag", func(t *testing.T) {
			assert.False(t, faded(tag(0)))
		})

		t.Run("it should fade another road's tag", func(t *testing.T) {
			assert.True(t, faded(tag(1)))
		})
	})

	t.Run("when the pointer is on nothing", func(t *testing.T) {
		scene.SetHover(city.Hit{})
		g.lit = scene.Highlight()

		t.Run("it should fade no badge", func(t *testing.T) {
			assert.False(t, faded(badge("c")))
		})
	})
}
