package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 212: a key hashes what a drawable is drawn from through a plan
// made once for its type, which must hash every value as walking it by
// reflection does.
func TestTheValueHash(t *testing.T) {
	type inner struct {
		n    int
		name string
	}
	type mixed struct {
		b      bool
		i8     int8
		u16    uint16
		f32    float32
		f      float64
		s      string
		arr    [3]int
		sl     []inner
		p      *inner
		nilp   *inner
		m      map[string]int
		at     time.Time
		any    any
		nested inner
		scale  *ebiten.ColorScale
	}
	scale := &ebiten.ColorScale{}
	scale.SetR(0.5)
	at := time.Date(2026, 10, 26, 1, 0, 0, 0, time.UTC)
	rc := newRecordCity(t)
	cam := rc.scene.Camera()
	var frames []any
	for _, b := range rc.scene.City().Buildings() {
		if f, ok := rc.g.buildingFrame(cam, b, false, true, 0); ok {
			f.b, f.seconds, f.dress = nil, 0, lot{}
			frames = append(frames, f)
		}
		frames = append(frames, b)
	}
	values := []any{
		0, 7, int64(-3), uint8(9), 1.5, float32(2.25), true, "a sign", "",
		[2]int{1, 2}, []string{"x", "y"}, []int(nil),
		inner{3, "three"}, &inner{4, "four"}, (*inner)(nil),
		mixed{b: true, i8: -2, u16: 600, f32: 0.75, f: 3.5, s: "s", arr: [3]int{1, 2, 3},
			sl: []inner{{1, "a"}, {2, "b"}}, p: &inner{5, "p"}, nilp: nil, m: map[string]int{"a": 1, "b": 2},
			at: at, any: inner{6, "any"}, nested: inner{7, "n"}, scale: scale},
		city.Rect{Min: city.Point{X: 1, Y: 2}, Max: city.Point{X: 3, Y: 4}},
		city.DistrictSign{Name: "Oak", Move: "next", At: city.Point{X: 5, Y: 6}},
	}
	values = append(values, frames...)
	for i, v := range values {
		t.Run(fmt.Sprintf("when %T number %d is hashed through its plan", v, i), func(t *testing.T) {
			t.Run("it should hash as walking it by reflection does", func(t *testing.T) {
				assert.Equal(t, reflectHash(v), valueHash(v))
			})
		})
	}
	for i, f := range frames {
		t.Run(fmt.Sprintf("when %T number %d is hashed where it is", f, i), func(t *testing.T) {
			v, ok := f.(buildingFrame)
			if !ok {
				return
			}
			t.Run("it should hash as copying it does", func(t *testing.T) {
				assert.Equal(t, valueHash(v), hashOf(&v))
			})
		})
	}

	t.Run("when two values differ in one field", func(t *testing.T) {
		a := mixed{s: "a", sl: []inner{{1, "a"}}}
		b := a
		b.sl = []inner{{1, "b"}}

		t.Run("it should hash them apart", func(t *testing.T) {
			assert.NotEqual(t, valueHash(a), valueHash(b))
		})
	})
}
