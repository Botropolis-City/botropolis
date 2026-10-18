package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/script"
)

func TestPlayer(t *testing.T) {
	building := city.Point{X: 800, Y: 400}
	resolve := func(target string) (city.Point, bool) {
		if target == "Heights in feet" {
			return building, true
		}
		return city.Point{}, false
	}
	frames := func(cues []script.Cue, seconds float64) []playFrame {
		p := newPlayer(cues, city.Point{X: 100, Y: 100})
		var out []playFrame
		for tick := 0; tick <= int(seconds*30); tick++ {
			out = append(out, p.step(float64(tick)/30, resolve))
		}
		return out
	}

	t.Run("when the pointer glides to a session over a second", func(t *testing.T) {
		got := frames([]script.Cue{{At: 1, PointAt: "Heights in feet", Over: 1}}, 3)

		t.Run("it should not move before its cue", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 100, Y: 100}, got[15].pointer)
		})

		t.Run("it should be partway there halfway through", func(t *testing.T) {
			assert.InDelta(t, 450, got[45].pointer.X, 1)
		})

		t.Run("it should arrive on the building", func(t *testing.T) {
			assert.Equal(t, building, got[60].pointer)
		})

		t.Run("it should stay with the building as the camera moves", func(t *testing.T) {
			building = city.Point{X: 900, Y: 420}
			moved := frames([]script.Cue{{At: 1, PointAt: "Heights in feet", Over: 1}}, 3)
			building = city.Point{X: 800, Y: 400}
			assert.Equal(t, city.Point{X: 900, Y: 420}, moved[80].pointer)
		})
	})

	t.Run("when a click is cued", func(t *testing.T) {
		got := frames([]script.Cue{{At: 1, Click: true}}, 2)

		t.Run("it should press on its frame", func(t *testing.T) {
			assert.True(t, got[30].press)
		})

		t.Run("it should release a few frames later, as a hand would", func(t *testing.T) {
			var released int
			for i, f := range got {
				if f.release {
					released = i
				}
			}
			assert.Equal(t, 33, released)
		})
	})

	t.Run("when an arrow is held for half a second", func(t *testing.T) {
		got := frames([]script.Cue{{At: 1, Hold: "ArrowRight", For: 0.5}}, 2)
		held := 0
		for _, f := range got {
			if f.hold == "ArrowRight" {
				held++
			}
		}

		t.Run("it should be down for fifteen frames", func(t *testing.T) {
			assert.Equal(t, 15, held)
		})
	})

	t.Run("when the wheel is turned three notches over a second", func(t *testing.T) {
		got := frames([]script.Cue{{At: 0, Wheel: 3, Over: 1}}, 2)
		var turns []float64
		for _, f := range got {
			if f.wheel != 0 {
				turns = append(turns, f.wheel)
			}
		}

		t.Run("it should turn it a whole notch at a time, since the scene rounds a turn to notches", func(t *testing.T) {
			assert.Equal(t, []float64{1, 1, 1}, turns)
		})
	})

	t.Run("when the wheel is turned back three notches", func(t *testing.T) {
		got := frames([]script.Cue{{At: 0, Wheel: -3, Over: 1}}, 2)
		var total float64
		for _, f := range got {
			total += f.wheel
		}

		t.Run("it should turn it back three", func(t *testing.T) {
			assert.Equal(t, -3.0, total)
		})
	})

	t.Run("when the camera pans to a session over a second", func(t *testing.T) {
		centre := city.Point{X: 960, Y: 540}
		target := city.Point{X: 400, Y: 900}
		var panned city.Point
		p := newPlayer([]script.Cue{{At: 0.5, PanTo: "Heights in feet", Over: 1}}, centre)
		p.centre = centre
		var halfway city.Point
		for tick := 0; tick <= 60; tick++ {
			f := p.step(float64(tick)/30, func(string) (city.Point, bool) { return target.Add(panned), true })
			panned = panned.Add(f.pan)
			if tick == 30 {
				halfway = target.Add(panned)
			}
		}

		t.Run("it should be on its way halfway through", func(t *testing.T) {
			assert.True(t, halfway.X > target.X && halfway.X < centre.X, "halfway at %v", halfway)
		})

		t.Run("it should end with the session in the middle of the window", func(t *testing.T) {
			got := target.Add(panned)
			assert.InDelta(t, centre.X, got.X, 0.5)
			assert.InDelta(t, centre.Y, got.Y, 0.5)
		})
	})

	t.Run("when a key is cued", func(t *testing.T) {
		got := frames([]script.Cue{{At: 0.5, Key: "equal"}}, 1)

		t.Run("it should press it on that frame alone", func(t *testing.T) {
			var on []int
			for i, f := range got {
				if f.key == "equal" {
					on = append(on, i)
				}
			}
			assert.Equal(t, []int{15}, on)
		})
	})

	t.Run("when a statistics prompt is cued", func(t *testing.T) {
		got := frames([]script.Cue{{At: 0.5, ShowStatsPrompt: "on"}}, 1)

		t.Run("it should pass it on that frame alone", func(t *testing.T) {
			var on []int
			for i, f := range got {
				if f.statsPrompt == "on" {
					on = append(on, i)
				}
			}
			assert.Equal(t, []int{15}, on)
		})
	})
}
