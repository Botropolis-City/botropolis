package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 212: a still drawable whose key holds keeps last frame's record,
// moved by the frame's whole-pixel pan, and is recorded afresh when
// anything it is drawn from changes.
func TestRecordReuse(t *testing.T) {
	// settled is a record city drawn until it keeps everything it can,
	// each kept record checked against one made afresh.
	settled := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 1
		rc.scene.SetView(city.ViewAttention)
		for range 3 {
			rc.frame()
		}
		require.Positive(t, rc.frame().reused, "precondition: the settled city keeps records")
		return rc
	}

	t.Run("when a city has settled, and 30 frames pass with nothing changing", func(t *testing.T) {
		rc := settled(t)
		var total stillStats
		for range 30 {
			st := rc.frame()
			total.fresh += st.fresh
			total.stale += st.stale
		}

		t.Run("it should record no still drawable afresh", func(t *testing.T) {
			assert.Zero(t, total.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, total.stale)
		})
	})

	t.Run("when the camera pans by whole pixels", func(t *testing.T) {
		rc := settled(t)
		rc.scene.Pan(city.Point{X: 7, Y: -3})
		st := rc.frame()

		t.Run("it should record no still drawable afresh", func(t *testing.T) {
			assert.Zero(t, st.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	for _, c := range []struct {
		name string
		move func(rc *recordCity)
	}{
		{"pans by part of a pixel", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 0.5, Y: 0.25}) }},
		{"zooms", func(rc *recordCity) { rc.scene.Camera().Zoom *= 1.25 }},
	} {
		t.Run("when the camera "+c.name, func(t *testing.T) {
			rc := settled(t)
			c.move(rc)
			st := rc.frame()

			t.Run("it should keep no record", func(t *testing.T) {
				assert.Zero(t, st.reused)
			})
		})
	}

	t.Run("when one working session's activity moves in a snapshot", func(t *testing.T) {
		rc := settled(t)
		rc.now = rc.now.Add(time.Second)
		rc.snap(func(s *state.Snapshot) {
			s.At = rc.now
			session(s, "s00").LastActivity = rc.now
		})
		st := rc.frame()

		t.Run("it should record afresh only that session's lot", func(t *testing.T) {
			assert.Equal(t, 1, st.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when the camera has panned, and a frame is drawn straight between two that are not, as the frame check draws them", func(t *testing.T) {
		rc := settled(t)
		rc.scene.Pan(city.Point{X: 5, Y: 4})
		rc.frame()
		rc.g.noStill = true
		rc.frame()
		rc.g.noStill = false
		st := rc.frame()

		t.Run("it should record no still drawable afresh", func(t *testing.T) {
			assert.Zero(t, st.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when reuse is off", func(t *testing.T) {
		rc := settled(t)
		rc.g.noReuse = true
		st := rc.frame()

		t.Run("it should keep no record", func(t *testing.T) {
			assert.Zero(t, st.reused)
		})
	})
}

// Item 212: what a settled frame of a city with aged lots costs with its
// still drawables' records kept, and recorded afresh. It reports and
// never gates.
func BenchmarkASettledFrameKeepingRecords(b *testing.B) {
	for _, c := range []struct {
		name  string
		fresh bool
	}{{"kept", false}, {"afresh", true}} {
		b.Run(c.name, func(b *testing.B) {
			rc := newRecordCity(b)
			rc.g.verifyEvery = releaseVerifyEvery
			rc.g.noReuse = c.fresh
			for range 3 {
				rc.frame()
			}
			b.ReportAllocs()
			for b.Loop() {
				rc.frame()
			}
		})
	}
}
