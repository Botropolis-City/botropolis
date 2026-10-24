package render

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 217: a delivery truck rides the deck at the deck's height from
// the first span out, whichever district it leaves and whatever the city
// did while it drove.
func TestDeliveryOnTheDeck(t *testing.T) {
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	session := func(id, cwd string, prs ...claude.PR) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.Working, StartedAt: start, PRs: prs}
	}
	pr := func(st string) claude.PR {
		return claude.PR{Number: 1, URL: "https://github.com/o/r/pull/1", Repository: "o/r", State: st}
	}
	city3 := func(n int, merging string, st string) []state.Session {
		out := []state.Session{session("m", merging, pr(st))}
		for i := 0; i < n; i++ {
			out = append(out, session(fmt.Sprintf("k%d", i), "/p/kiln"))
		}
		return append(out, session("t", "/p/tidepool"), session("o", "/p/orchard"))
	}
	// lowOnTheDeck drives the truck to the end in small steps and returns
	// every place it stood past the ramp without the deck's full height.
	lowOnTheDeck := func(s *city.Scene, clock *time.Time, grow func(d time.Duration)) []city.Point {
		var low []city.Point
		for d := time.Duration(0); ; d += 50 * time.Millisecond {
			*clock = start.Add(d)
			grow(d)
			trucks := s.Deliveries()
			if len(trucks) == 0 {
				return low
			}
			b := s.City().Bridge()
			at, _ := deliveryPlace(trucks[0], *clock)
			past := (at.X - b.Land.X) * math.Copysign(1, b.Far.X-b.Land.X)
			if past > b.RampRun() && b.Rise(at) < 1 {
				low = append(low, at)
			}
		}
	}

	for _, c := range []struct {
		name    string
		merging string
		grows   bool
	}{
		{"leaves kiln", "/p/kiln", false},
		{"leaves tidepool", "/p/tidepool", false},
		{"leaves orchard", "/p/orchard", false},
		{"leaves tidepool and the island grows a row while it drives", "/p/tidepool", true},
	} {
		t.Run("when a truck "+c.name, func(t *testing.T) {
			clock := start
			s := city.NewScene(city.NewLayout())
			s.Resize(1920, 1080)
			s.SetClock(func() time.Time { return clock }, time.UTC)
			s.SetSnapshot(state.Snapshot{At: start, Sessions: city3(3, c.merging, claude.PROpen)})
			s.SetSnapshot(state.Snapshot{At: start, Sessions: city3(3, c.merging, claude.PRMerged)})
			require.Len(t, s.Deliveries(), 1)
			was := s.City().Bridge()
			grown := false
			low := lowOnTheDeck(s, &clock, func(d time.Duration) {
				if c.grows && !grown && d >= 2*time.Second {
					grown = true
					s.SetSnapshot(state.Snapshot{At: start, Sessions: city3(6, c.merging, claude.PRMerged)})
					require.NotEqual(t, was.Land, s.City().Bridge().Land, "the fixture should move the bridge")
				}
			})

			t.Run("it should be drawn at the deck's full height everywhere past the ramp", func(t *testing.T) {
				assert.Empty(t, low)
			})
		})
	}
}
