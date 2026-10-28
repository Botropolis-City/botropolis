package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 212: at full zoom, with six sessions working, the quiet pair at
// r1192 spent 33.5 s of 60 baking boarded facades again every frame: a
// look the pages had no room to keep was evicted by the next bake, found
// missing, baked again, and each bake made its building's record afresh.
// A look evicted soon after its bake is drawn straight, which is exact,
// for lookBackoff frames, and its record kept.
func TestLookThrash(t *testing.T) {
	// thrashing is a settled city whose look cache holds about half its
	// looks, so each bake evicts another.
	thrashing := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyBudget = 0
		rc.scene.SetView(city.ViewAttention)
		rc.frame()
		rc.now = rc.now.Add(10 * time.Minute)
		for range 3 {
			rc.frame()
		}
		all := rc.g.looks.bytes()
		require.Positive(t, all, "precondition: the city's looks are baked")
		rc.g.looks = newLookCache[lookKey, *bakedLook](all / 2)
		return rc
	}
	buildings := func(rc *recordCity) int { return len(rc.scene.City().Buildings()) }
	// frames draws n frames and is how many looks they baked and how
	// many building records they made afresh.
	frames := func(rc *recordCity, n int) (bakes, fresh int) {
		for range n {
			st := rc.frame()
			// The look stats are each frame's own.
			bakes += rc.g.lookStats.bakes
			fresh += st.freshBy[kindHouse]
		}
		return bakes, fresh
	}

	t.Run("when the look pages hold about half the city's looks, so bakes evict one another", func(t *testing.T) {
		rc := thrashing(t)
		// The frames the backoffs begin on draw what thrashed straight.
		var straight, misses int
		for range 4 {
			rc.frame()
			straight += rc.g.lookStats.straight
		}
		bakes := 0
		for range lookBackoff / 2 {
			rc.frame()
			bakes += rc.g.lookStats.bakes
			misses += rc.g.lookStats.misses
		}
		st := rc.frame()
		held := drawn(rc.g)

		t.Run("it should count a backed-off look's drawing as straight", func(t *testing.T) {
			assert.Positive(t, straight)
		})
		t.Run("it should count no miss while it backs off", func(t *testing.T) {
			assert.Zero(t, misses)
		})

		t.Run("it should bake no look again within its backoff", func(t *testing.T) {
			assert.LessOrEqual(t, bakes, buildings(rc))
		})
		t.Run("it should keep every still building's record once the thrash is backed off", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindHouse])
		})
		t.Run("it should hold the frame made afresh", func(t *testing.T) {
			assert.Equal(t, afresh(rc), held)
		})
	})

	t.Run("when the pressure never lifts", func(t *testing.T) {
		rc := thrashing(t)
		_, first := frames(rc, 4*lookBackoff)
		_, later := frames(rc, 4*lookBackoff)

		// When a backoff ends a building's record is made a few times
		// over: as its key changes, once its bake has unsettled it, and
		// when that bake is evicted and it is drawn straight again.
		t.Run("it should make each building's record a few times a backoff at most", func(t *testing.T) {
			assert.LessOrEqual(t, first, 4*4*buildings(rc))
		})
		t.Run("it should make fewer records as the pressure goes on, its backoffs growing", func(t *testing.T) {
			assert.Less(t, later, first)
		})
	})

	t.Run("when a building's backoff ends", func(t *testing.T) {
		rc := thrashing(t)
		frames(rc, 2)
		var id string
		for b, until := range rc.g.backoffUntil {
			id = b
			for rc.g.lookFrame < until {
				rc.frame()
			}
			break
		}
		require.NotEmpty(t, id, "precondition: a building backs off")

		t.Run("it should say so, so its look's miss is not taken for an eviction", func(t *testing.T) {
			assert.True(t, rc.g.backoffEnded(id))
		})
		t.Run("and a frame later, it should not", func(t *testing.T) {
			rc.frame()
			assert.False(t, rc.g.backoffEnded(id))
		})
	})

	t.Run("when the pressure lifts, so the looks fit again", func(t *testing.T) {
		rc := thrashing(t)
		frames(rc, lookBackoff/2)
		rc.g.looks.cap = 1 << 30
		rebaked, fresh := frames(rc, lookBackoff+4)
		bakes, after := frames(rc, 4)

		t.Run("it should make each building's record afresh a few times at most on the way", func(t *testing.T) {
			assert.LessOrEqual(t, fresh, 4*buildings(rc))
		})
		t.Run("it should bake the looks again once the backoff ends", func(t *testing.T) {
			assert.Positive(t, rebaked)
		})
		t.Run("it should then bake nothing more", func(t *testing.T) {
			assert.Zero(t, bakes)
		})
		t.Run("it should then keep every record", func(t *testing.T) {
			assert.Zero(t, after)
		})
	})
}
