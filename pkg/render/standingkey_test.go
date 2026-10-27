package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 212: the standing key is what a frame reuses the still city by,
// so it must move with everything a still thing is drawn from, and hold
// while nothing is.
func TestTheStandingKey(t *testing.T) {
	settled := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.frame()
		// Past the flags' hoisting and the course's glow, which move.
		rc.now = rc.now.Add(10 * time.Minute)
		for range 3 {
			rc.frame()
		}
		return rc
	}
	key := func(rc *recordCity) uint64 {
		rc.frame()
		return rc.g.still.standing
	}

	t.Run("when a settled city is drawn again with nothing changed", func(t *testing.T) {
		rc := settled(t)
		before := key(rc)
		rc.now = rc.now.Add(time.Second)

		t.Run("it should hold", func(t *testing.T) {
			assert.Equal(t, before, key(rc))
		})
	})

	for _, c := range []struct {
		name   string
		change func(rc *recordCity)
		holds  bool
	}{
		{"the snapshot is taken again", func(rc *recordCity) { rc.snap(func(*state.Snapshot) {}) }, false},
		{"the camera zooms", func(rc *recordCity) { rc.scene.Camera().Zoom *= 1.25 }, false},
		{"the camera turns", func(rc *recordCity) { rc.scene.Turn() }, false},
		{"the camera pans by part of a pixel", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 0.4}) }, false},
		{"night falls", func(rc *recordCity) {
			for night := rc.scene.City().Night; rc.scene.City().Night == night; {
				rc.scene.CycleLight()
			}
		}, false},
		{"the view changes", func(rc *recordCity) { rc.scene.SetView(city.ViewSpend) }, false},
		{"a search dims buildings", func(rc *recordCity) { rc.scene.SetFilter("s01") }, false},
		{"the labels are hidden", func(rc *recordCity) { rc.scene.SetLabels(false) }, false},
		{"a building is selected", func(rc *recordCity) { rc.scene.Select("s01") }, false},
		{"the city is made to stand still", func(rc *recordCity) { rc.g.setLiveliness(city.Still) }, false},
		{"the camera pans by whole pixels", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 9, Y: -7}) }, true},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			rc := settled(t)
			before := key(rc)
			c.change(rc)
			after := key(rc)

			if c.holds {
				t.Run("it should hold, the still city moved whole", func(t *testing.T) {
					assert.Equal(t, before, after)
				})
				return
			}
			t.Run("it should change", func(t *testing.T) {
				assert.NotEqual(t, before, after)
			})
		})
	}

	// Each look the clock changes, begun by a snapshot as the city
	// begins it: its key is read a frame a second from steady until past
	// the moment it changes.
	for _, c := range []struct {
		name string
		// start begins the look on a settled city at its clock.
		start func(rc *recordCity)
		// steady is how long after start the look stands still, and
		// changes when it has changed.
		steady, changes time.Duration
		// runs is whether it moves every frame until it changes, so no
		// frame before it holds.
		runs bool
	}{
		{"a course's glow ends", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.Sessions[3].Compactions++
				s.Sessions[3].LastCompactionAt = rc.now
			})
		}, 5 * time.Second, courseGlow + 2*time.Second, false},
		{"a celebration ends", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.Sessions[6].PRs = append(s.Sessions[6].PRs, claude.PR{Number: 3, URL: "https://example.com/pr/3", State: claude.PRMerged})
			})
		}, 0, city.CelebrateFor + 2*time.Second, false},
		{"a flag finishes its climb", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.At = rc.now
				s.Sessions[6].PRs = append(s.Sessions[6].PRs, claude.PR{Number: 4, URL: "https://example.com/pr/4", State: claude.PROpen})
			})
		}, 0, city.FlagFor + 2*time.Second, true},
		{"a building wakes and settles", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.Sessions[1].State, s.Sessions[1].LastActivity = state.Working, rc.now
			})
		}, 0, 3 * time.Minute, true},
		// Item 280: the clinic's plate counts what needs you, and a held
		// frame replays last frame's plate.
		{"a snooze runs out and the clinic counts one more", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.Sessions[1].State, s.Sessions[1].LastActivity = state.NeedsYou, rc.now.Add(-time.Hour)
			})
			rc.scene.Layout().Snoozed = map[string]time.Time{rc.snapshot.Sessions[1].ID: rc.now.Add(time.Minute)}
		}, 30 * time.Second, time.Minute + 2*time.Second, false},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			rc := settled(t)
			c.start(rc)
			began := rc.now
			var keys []uint64
			var at []time.Duration
			for d := time.Duration(0); d <= 3*time.Minute; d += time.Second {
				rc.now = began.Add(d)
				keys = append(keys, key(rc))
				at = append(at, d)
			}
			last := len(keys) - 1
			changed := -1
			for i := 1; i <= last; i++ {
				if keys[i] != keys[i-1] && at[i] > c.steady {
					changed = i
					break
				}
			}
			t.Logf("changes at %v", func() any {
				if changed < 0 {
					return "never"
				}
				return at[changed]
			}())
			require.Positive(t, changed, "precondition: the look changes after it is steady")

			t.Run("it should change the frame its look changes", func(t *testing.T) {
				assert.LessOrEqual(t, at[changed], c.changes)
			})
			if c.runs {
				t.Run("it should change every frame while it runs", func(t *testing.T) {
					assert.NotEqual(t, keys[changed-1], keys[changed])
				})
			} else {
				t.Run("it should hold the frame before", func(t *testing.T) {
					assert.Equal(t, keys[changed-2], keys[changed-1])
				})
			}
			t.Run("it should hold once it has changed", func(t *testing.T) {
				assert.Equal(t, keys[last-1], keys[last])
			})
		})
	}
}
