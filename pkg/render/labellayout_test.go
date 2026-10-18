package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 102: badges, station labels and district signs are laid out
// together, so none of them covers another.
func TestLayoutLabels(t *testing.T) {
	const gap = 2.0
	badge := func(x, y, anchorY float64) mapLabel {
		return mapLabel{Kind: labelBadge, Rect: city.RectAt(x, y, 60, 12), Anchor: city.Point{X: x + 30, Y: anchorY}}
	}
	station := func(x, y float64) mapLabel {
		return mapLabel{Kind: labelStation, Rect: city.RectAt(x, y, 60, 12), Anchor: city.Point{X: x, Y: y + 20}}
	}
	sign := func(r city.Rect) mapLabel {
		return mapLabel{Kind: labelSign, Rect: r, Anchor: r.Center()}
	}
	type check func(t *testing.T, in []mapLabel, out []laidLabel)
	type then struct {
		should string
		check  check
	}
	stays := func(i int) check {
		return func(t *testing.T, in []mapLabel, out []laidLabel) {
			assert.Equal(t, laidLabel{At: in[i].Rect, Shown: true}, out[i])
		}
	}
	steps := func(i int) check {
		return func(t *testing.T, in []mapLabel, out []laidLabel) {
			assert.True(t, out[i].Shown && out[i].At != in[i].Rect, "shown somewhere else")
		}
	}

	cases := []struct {
		name   string
		labels []mapLabel
		then   []then
	}{
		{"a badge lands on a district sign", []mapLabel{sign(city.RectAt(0, 0, 80, 30)), badge(10, 20, 15)}, []then{
			{"it should move the badge off the sign and still draw it, because there is room beside it", steps(1)},
		}},
		{"two badges land on each other", []mapLabel{badge(10, 4, 5), badge(0, 0, 10)}, []then{
			{"it should keep the front-most where it asked to be", stays(1)},
			{"it should step the other aside or down, and still draw it", steps(0)},
		}},
		{"a station label lands on a badge", []mapLabel{badge(5, 3, 0), station(0, 0)}, []then{
			{"it should keep the station label where it asked to be", stays(1)},
			{"it should step the badge off it, and still draw it", steps(0)},
		}},
		{"a badge is moved out past its first ring of neighbours", []mapLabel{sign(city.RectAt(-70, -20, 200, 48)), badge(0, 0, 0)}, []then{
			{"it should carry a leader back to its building", func(t *testing.T, _ []mapLabel, out []laidLabel) {
				assert.True(t, out[1].Shown && out[1].Leader, "shown with a leader")
			}},
		}},
		{"a badge is boxed in on every side", []mapLabel{sign(city.RectAt(-200, -200, 400, 400)), badge(0, 0, 0)}, []then{
			{"it should be left off", func(t *testing.T, _ []mapLabel, out []laidLabel) {
				assert.False(t, out[1].Shown)
			}},
		}},
		{"nothing collides", []mapLabel{sign(city.RectAt(0, 0, 80, 30)), badge(200, 0, 0), badge(0, 200, 0), station(200, 200)}, []then{
			{"it should leave every label where it asked to be, without a leader", func(t *testing.T, in []mapLabel, out []laidLabel) {
				var moved []string
				for i, l := range out {
					if l != (laidLabel{At: in[i].Rect, Shown: true}) {
						moved = append(moved, in[i].Kind.String())
					}
				}
				assert.Empty(t, moved)
			}},
		}},
	}

	for _, c := range cases {
		out := layoutLabels(c.labels, gap)
		t.Run("when "+c.name, func(t *testing.T) {
			require.Len(t, out, len(c.labels))

			t.Run("it should leave no two drawn labels overlapping", func(t *testing.T) {
				var overlaps []string
				for i := range out {
					for j := i + 1; j < len(out); j++ {
						if out[i].Shown && out[j].Shown && out[i].At.Overlaps(out[j].At) {
							overlaps = append(overlaps, c.labels[i].Kind.String()+" on "+c.labels[j].Kind.String())
						}
					}
				}
				assert.Empty(t, overlaps)
			})

			t.Run("it should leave every district sign where it stood", func(t *testing.T) {
				var moved []city.Rect
				for i, l := range out {
					if c.labels[i].Kind == labelSign && l != (laidLabel{At: c.labels[i].Rect, Shown: true}) {
						moved = append(moved, l.At)
					}
				}
				assert.Empty(t, moved)
			})

			for _, th := range c.then {
				t.Run(th.should, func(t *testing.T) {
					th.check(t, c.labels, out)
				})
			}
		})
	}
}
