package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 222: a vehicle keeps a lot lane's half-lane inside a block and a
// road's on the avenues, so whether a place is inside a block has one
// definition.
func TestInBlock(t *testing.T) {
	c := city.Build(state.Snapshot{At: now, Sessions: sixDistricts(), Roads: roadsBetween([2]string{"a", "d"})}, city.NewLayout())
	a := districtNamed(c, "a")
	require.NotNil(t, a, "precondition: district a")
	dw, ok := c.Driveway(a)
	require.True(t, ok, "precondition: a has a driveway")

	for _, at := range []struct {
		name string
		p    city.Point
		want bool
	}{
		{"the middle of a lot", a.Rect.Center(), true},
		{"a lot's driveway end", dw.Lot, true},
		{"just inside a lot's edge", city.Point{X: a.Rect.Center().X, Y: a.Rect.Max.Y - 0.5}, true},
		{"the avenue cell of a driveway", dw.Cell.Center(), false},
		{"just outside a lot's edge", city.Point{X: a.Rect.Center().X, Y: a.Rect.Max.Y + 0.5}, false},
	} {
		t.Run("when a place is "+at.name, func(t *testing.T) {
			t.Run("it should say whether it is inside a block", func(t *testing.T) {
				assert.Equal(t, at.want, c.InBlock(at.p))
			})
		})
	}
}

// Item 222: a vehicle along a lot's front drives far enough in from its
// edge that its nose stays on the lot through a turn.
func TestFrontLane(t *testing.T) {
	c := city.Build(state.Snapshot{At: now, Sessions: sixDistricts(), Roads: roadsBetween([2]string{"a", "d"})}, city.NewLayout())
	a := districtNamed(c, "a")
	require.NotNil(t, a, "precondition: district a")
	dw, ok := c.Driveway(a)
	require.True(t, ok, "precondition: a has a driveway")

	t.Run("when a lot's driveway is laid", func(t *testing.T) {
		t.Run("it should end the driveway on the lot's front lane", func(t *testing.T) {
			assert.Equal(t, a.Rect.Max.Y-city.FrontLane, dw.Lot.Y)
		})
	})
}
