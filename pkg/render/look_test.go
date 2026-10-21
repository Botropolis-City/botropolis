package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 192: a building is drawn as steps in its paint order, and each
// run of steps that is not moving is baked into one image.
func TestBuildingSteps(t *testing.T) {
	still := lookState{courses: true, facade: true, beacon: true, sign: true, flags: 1, selected: true, skip: true, work: true, spire: true}

	for _, c := range []struct {
		when string
		set  func(*lookState)
		step step
	}{
		{"a course is pouring", func(s *lookState) { s.pouring = true }, stepCourses},
		{"it is rising", func(s *lookState) { s.rising = true }, stepSprite},
		{"it is fading in off a truck", func(s *lookState) { s.fading = true }, stepSprite},
		{"it is settling", func(s *lookState) { s.settling = true }, stepSprite},
		{"it is settling, its windows", func(s *lookState) { s.settling = true }, stepFacade},
		{"it is being built", func(s *lookState) { s.building = true }, stepSprite},
		{"it is being built, its site", func(s *lookState) { s.building = true }, stepSite},
		{"its beacon pulses", func(s *lookState) { s.pulse = true }, stepBeacon},
		{"its beacon is fading with a settle", func(s *lookState) { s.settling = true }, stepBeacon},
		{"a flag is being hoisted", func(s *lookState) { s.hoisting = true }, stepFlags},
		{"it is celebrating", func(s *lookState) { s.celebrating = true }, stepCelebrate},
		{"it is smoking", func(s *lookState) { s.smoke = true }, stepSmoke},
		{"trucks are at its skip", func(s *lookState) { s.trucks = true }, stepSkipTrucks},
		{"loads are on its hoist", func(s *lookState) { s.loads = true }, stepWork},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			s := still
			c.set(&s)
			present, live := s.steps()

			t.Run("it should draw that step", func(t *testing.T) {
				assert.True(t, present[c.step])
			})

			t.Run("it should draw that step live", func(t *testing.T) {
				assert.True(t, live[c.step])
			})
		})
	}

	t.Run("when nothing moves", func(t *testing.T) {
		runs := runsOf(still.steps())

		t.Run("it should bake every step into one run", func(t *testing.T) {
			assert.Equal(t, []run{{from: stepCourses, to: stepCount, baked: true}}, runs)
		})
	})

	t.Run("when nothing moves and its beacon pulses", func(t *testing.T) {
		s := still
		s.pulse = true
		runs := runsOf(s.steps())

		t.Run("it should bake two runs either side of the beacon", func(t *testing.T) {
			assert.Equal(t, []run{
				{from: stepCourses, to: stepBeacon, baked: true},
				{from: stepBeacon, to: stepSign},
				{from: stepSign, to: stepCount, baked: true},
			}, runs)
		})
	})

	t.Run("when nothing moves and its lights are on", func(t *testing.T) {
		s := still
		s.lights = true
		runs := runsOf(s.steps())

		t.Run("it should note its lights between two baked runs", func(t *testing.T) {
			assert.Equal(t, []run{
				{from: stepCourses, to: stepLights, baked: true},
				{from: stepLights, to: stepBeacon},
				{from: stepBeacon, to: stepCount, baked: true},
			}, runs)
		})
	})

	t.Run("when its lights are on and its beacon pulses", func(t *testing.T) {
		s := still
		s.lights, s.pulse = true, true
		runs := runsOf(s.steps())

		t.Run("it should keep its lights a run of their own", func(t *testing.T) {
			assert.Equal(t, []run{
				{from: stepCourses, to: stepLights, baked: true},
				{from: stepLights, to: stepBeacon},
				{from: stepBeacon, to: stepSign},
				{from: stepSign, to: stepCount, baked: true},
			}, runs)
		})
	})

	t.Run("when it is dimmed", func(t *testing.T) {
		s := still
		s.stop = true
		present, _ := s.steps()

		t.Run("it should draw nothing over its walls", func(t *testing.T) {
			assert.False(t, present[stepFacade] || present[stepBeacon] || present[stepSign] || present[stepFlags] || present[stepSelected])
		})

		t.Run("it should still draw its courses", func(t *testing.T) {
			assert.True(t, present[stepCourses])
		})
	})

	t.Run("when it has no skip, work or spire", func(t *testing.T) {
		s := lookState{}
		present, _ := s.steps()

		t.Run("it should draw only its sprite", func(t *testing.T) {
			var want [stepCount]bool
			want[stepSprite] = true
			assert.Equal(t, want, present)
		})
	})
}

// A look's key holds while the camera pans by whole pixels, and changes
// with anything that changes how the building looks.
func TestLookKey(t *testing.T) {
	foot := city.Point{X: 100.25, Y: 40.5}
	base := lookKey{id: "s", piece: "office-3", heading: 90, zoom: 1.5, compactions: 2, title: "fix the build", spire: 1}
	keyAt := func(k lookKey, at city.Point) lookKey {
		k.phase = lookPhase(at)
		return k
	}

	t.Run("when the camera pans by whole pixels", func(t *testing.T) {
		t.Run("it should keep the key", func(t *testing.T) {
			assert.Equal(t, keyAt(base, foot), keyAt(base, city.Point{X: foot.X + 37, Y: foot.Y - 12}))
		})
	})

	t.Run("when the camera pans by a whole pixel, give or take a rounding", func(t *testing.T) {
		t.Run("it should keep the key", func(t *testing.T) {
			assert.Equal(t, keyAt(base, foot), keyAt(base, city.Point{X: foot.X + 3 + 1e-9, Y: foot.Y - 1e-9}))
		})
	})

	t.Run("when the camera pans by a fraction of a pixel", func(t *testing.T) {
		t.Run("it should change the key", func(t *testing.T) {
			assert.NotEqual(t, keyAt(base, foot), keyAt(base, city.Point{X: foot.X + 0.5, Y: foot.Y}))
		})
	})
}
