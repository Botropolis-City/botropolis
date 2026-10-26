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

type fakeMouse struct {
	pressed, down, released map[ebiten.MouseButton]bool
}

func (m fakeMouse) JustPressed(b ebiten.MouseButton) bool  { return m.pressed[b] }
func (m fakeMouse) Pressed(b ebiten.MouseButton) bool      { return m.down[b] }
func (m fakeMouse) JustReleased(b ebiten.MouseButton) bool { return m.released[b] }

func pressOf(b ebiten.MouseButton) fakeMouse {
	return fakeMouse{pressed: map[ebiten.MouseButton]bool{b: true}, down: map[ebiten.MouseButton]bool{b: true}}
}

func holdOf(b ebiten.MouseButton) fakeMouse {
	return fakeMouse{down: map[ebiten.MouseButton]bool{b: true}}
}

func releaseOf(b ebiten.MouseButton) fakeMouse {
	return fakeMouse{released: map[ebiten.MouseButton]bool{b: true}}
}

type buttonPanRig struct {
	g        *Game
	actor    *recordingActor
	selected *city.Building
}

func newButtonPanRig(t *testing.T) *buttonPanRig {
	t.Helper()
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.SetInstant(true)
	s.Resize(1600, 1200)
	s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{
		{ID: "a", CWD: "/w/lib", State: state.Working},
		{ID: "b", CWD: "/w/api", State: state.Working},
		{ID: "c", CWD: "/w/web", State: state.Resting},
	}})
	require.True(t, s.Select("a"))
	actor := &recordingActor{}
	g := &Game{scene: s, actor: actor, scripted: -1, clicked: -1}
	return &buttonPanRig{g: g, actor: actor, selected: s.Selected()}
}

func (r *buttonPanRig) openGround(t *testing.T) city.Point {
	t.Helper()
	min := r.g.scene.City().Districts[0].Rect.Min
	for _, d := range r.g.scene.City().Districts {
		min.X = minFloat(min.X, d.Rect.Min.X)
		min.Y = minFloat(min.Y, d.Rect.Min.Y)
	}
	at := r.g.scene.Camera().WorldToScreen(city.Point{X: min.X - 3, Y: min.Y - 3})
	r.g.scene.PointerMove(at)
	require.Nil(t, r.g.scene.Hover().District)
	return at
}

func (r *buttonPanRig) districtFloor(t *testing.T) city.Point {
	t.Helper()
	for _, d := range r.g.scene.City().Districts {
		if r.selected != nil && d == r.g.scene.City().DistrictOf(r.selected) {
			continue
		}
		p := city.Point{X: d.Rect.Min.X + 1, Y: d.Rect.Min.Y + 1}
		for _, b := range d.Buildings {
			if b.Rect.Contains(p) {
				p = city.Point{X: d.Rect.Max.X - 1, Y: d.Rect.Max.Y - 1}
			}
		}
		at := r.g.scene.Camera().WorldToScreen(p)
		r.g.scene.PointerMove(at)
		if h := r.g.scene.Hover(); h.District != nil && h.Building == nil {
			return at
		}
	}
	t.Fatal("no district floor to press on")
	return city.Point{}
}

func (r *buttonPanRig) selectedBuilding(t *testing.T) city.Point {
	t.Helper()
	at := r.g.scene.Camera().WorldToScreen(r.selected.Rect.Center())
	r.g.scene.PointerMove(at)
	require.Equal(t, r.selected, r.g.scene.Hover().Building)
	return at
}

func (r *buttonPanRig) districtRects() map[string]city.Rect {
	rects := map[string]city.Rect{}
	for _, d := range r.g.scene.City().Districts {
		rects[d.Root] = d.Rect
	}
	return rects
}

func (r *buttonPanRig) drag(b ebiten.MouseButton, from, by city.Point) {
	to := from.Add(by)
	r.g.scene.PointerMove(from)
	r.g.buttonPan(from, pressOf(b))
	r.g.scene.PointerMove(to)
	r.g.buttonPan(to, holdOf(b))
	r.g.buttonPan(to, releaseOf(b))
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func TestButtonPan(t *testing.T) {
	buttons := []struct {
		name   string
		button ebiten.MouseButton
	}{
		{"right button", ebiten.MouseButtonRight},
		{"middle button", ebiten.MouseButtonMiddle},
	}
	places := []struct {
		name string
		at   func(*buttonPanRig, *testing.T) city.Point
	}{
		{"open ground", (*buttonPanRig).openGround},
		{"a district's floor", (*buttonPanRig).districtFloor},
		{"the selected building", (*buttonPanRig).selectedBuilding},
	}
	by := city.Point{X: 60, Y: 25}

	for _, button := range buttons {
		for _, place := range places {
			t.Run("when the "+button.name+" drags from "+place.name, func(t *testing.T) {
				r := newButtonPanRig(t)
				from := place.at(r, t)
				offset := r.g.scene.Camera().Offset
				rects := r.districtRects()
				r.g.scene.PointerMove(from)
				r.g.buttonPan(from, pressOf(button.button))
				r.g.scene.PointerMove(from.Add(by))
				r.g.buttonPan(from.Add(by), holdOf(button.button))
				_, pickedUp := r.g.scene.DistrictDrag()
				carrying := r.g.carrying
				r.g.buttonPan(from.Add(by), releaseOf(button.button))

				t.Run("it should pan the map", func(t *testing.T) {
					assert.NotEqual(t, offset, r.g.scene.Camera().Offset)
				})

				t.Run("it should leave every district where it was", func(t *testing.T) {
					assert.Equal(t, rects, r.districtRects())
				})

				t.Run("it should move no session", func(t *testing.T) {
					assert.Empty(t, r.actor.done)
				})

				t.Run("it should pick up no district", func(t *testing.T) {
					assert.False(t, pickedUp)
				})

				t.Run("it should carry nothing", func(t *testing.T) {
					assert.Nil(t, carrying)
				})

				t.Run("it should leave the selection as it was", func(t *testing.T) {
					assert.Equal(t, r.selected, r.g.scene.Selected())
				})
			})
		}

		t.Run("when the "+button.name+" is pressed and let go without moving", func(t *testing.T) {
			r := newButtonPanRig(t)
			at := r.districtFloor(t)
			r.drag(button.button, at, city.Point{})

			t.Run("it should leave the selection as it was", func(t *testing.T) {
				assert.Equal(t, r.selected, r.g.scene.Selected())
			})
		})

		t.Run("when the "+button.name+" drags while the city is under a scrim", func(t *testing.T) {
			r := newButtonPanRig(t)
			at := r.openGround(t)
			r.g.settingsOpen = true
			offset := r.g.scene.Camera().Offset
			r.drag(button.button, at, by)

			t.Run("it should not pan the map", func(t *testing.T) {
				assert.Equal(t, offset, r.g.scene.Camera().Offset)
			})
		})
	}

	t.Run("when the left button drags", func(t *testing.T) {
		r := newButtonPanRig(t)
		at := r.openGround(t)
		offset := r.g.scene.Camera().Offset
		r.drag(ebiten.MouseButtonLeft, at, by)

		t.Run("it should not pan through this path", func(t *testing.T) {
			assert.Equal(t, offset, r.g.scene.Camera().Offset)
		})
	})
}

func TestPanningHelp(t *testing.T) {
	t.Run("when the help lists the bindings", func(t *testing.T) {
		t.Run("its first row should name the right and middle drags", func(t *testing.T) {
			assert.Equal(t, "right or middle drag", bindings[0].Key)
		})

		t.Run("it should say alt+drag may be taken by the window manager", func(t *testing.T) {
			verb, _ := action(bindings, "shift+drag")
			assert.Contains(t, verb, "unless the window manager takes alt")
		})
	})

	t.Run("when the footer lists its keys", func(t *testing.T) {
		t.Run("it should offer the right drag to pan", func(t *testing.T) {
			verb, _ := action(footerKeys, "right drag")
			assert.Equal(t, "pan", verb)
		})
	})
}
