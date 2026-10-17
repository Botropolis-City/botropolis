package render

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 86: what a building resting or waking shows at each point of its
// transition: its lights go out window by window, from the top.
func TestPanesLit(t *testing.T) {
	const panes = 4
	for _, c := range []struct {
		name string
		kind city.Settle
		p    float64
		lit  int
	}{
		{"resting, at the start", city.SettleRest, 0, 4},
		{"resting, halfway", city.SettleRest, 0.5, 2},
		{"resting, at the end", city.SettleRest, 1, 0},
		{"waking, at the start", city.SettleWake, 0, 0},
		{"waking, halfway", city.SettleWake, 0.5, 2},
		{"waking, at the end", city.SettleWake, 1, 4},
	} {
		t.Run("when a building is "+c.name, func(t *testing.T) {
			t.Run(fmt.Sprintf("it should have %d of %d panes lit", c.lit, panes), func(t *testing.T) {
				assert.Equal(t, c.lit, panesLit(c.kind, c.p, panes))
			})
		})
	}
}

func TestBeaconFade(t *testing.T) {
	for _, c := range []struct {
		name  string
		kind  city.Settle
		p     float64
		alpha float64
	}{
		{"resting, at the start", city.SettleRest, 0, 1},
		{"resting, at the end", city.SettleRest, 1, 0},
		{"waking, at the start", city.SettleWake, 0, 0},
		{"waking, at the end", city.SettleWake, 1, 1},
	} {
		t.Run("when a building is "+c.name, func(t *testing.T) {
			t.Run(fmt.Sprintf("it should show its beacon at %g", c.alpha), func(t *testing.T) {
				assert.Equal(t, c.alpha, beaconFade(c.kind, c.p))
			})
		})
	}
}

func TestOutFirst(t *testing.T) {
	low := [4]city.Point{{X: 0, Y: 50}, {X: 10, Y: 55}, {X: 10, Y: 65}, {X: 0, Y: 60}}
	high := [4]city.Point{{X: 0, Y: 10}, {X: 10, Y: 15}, {X: 10, Y: 25}, {X: 0, Y: 20}}
	middle := [4]city.Point{{X: 20, Y: 30}, {X: 30, Y: 25}, {X: 30, Y: 35}, {X: 20, Y: 40}}

	t.Run("when a building's lights go out", func(t *testing.T) {
		t.Run("it should put out the top pane first and the lowest last", func(t *testing.T) {
			assert.Equal(t, []int{1, 2, 0}, outFirst([][4]city.Point{low, high, middle}))
		})
	})
}

func TestSettleTint(t *testing.T) {
	for _, c := range []struct {
		name string
		kind city.Settle
		p    float64
		want float32
	}{
		{"resting, at the start", city.SettleRest, 0, 1},
		{"resting, at the end", city.SettleRest, 1, 0.7},
		{"waking, at the start", city.SettleWake, 0, 0.7},
		{"waking, at the end", city.SettleWake, 1, 1},
	} {
		t.Run("when a building is "+c.name, func(t *testing.T) {
			t.Run(fmt.Sprintf("it should light its walls at %g", c.want), func(t *testing.T) {
				assert.InDelta(t, c.want, settleTint(c.kind, c.p).R(), 1e-6)
			})
		})
	}
}
