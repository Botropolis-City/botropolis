package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// kept holds a tint past the call that made it.
var kept *ebiten.ColorScale

// Item 212: the quiet pair at r1164 put the collector at 2.6% of fit
// against 1.8%. What runs for every building every frame allocates
// nothing.
func TestTheStandingKeysAllocations(t *testing.T) {
	rc := newRecordCity(t)
	rc.scene.SetView(city.ViewAttention)
	rc.snap(func(s *state.Snapshot) {
		// A session just begun, so a building has a site.
		s.Sessions = append(s.Sessions, state.Session{ID: "s99", CWD: "/src/p0", State: state.Working,
			StartedAt: rc.now, LastActivity: rc.now})
	})
	rc.frame()
	rc.now = rc.now.Add(time.Second)
	rc.frame()

	for _, c := range []struct {
		name string
		run  func()
	}{
		{"the standing key", func() {
			rc.g.frameBase.ok = false
			rc.g.standingKey(nil)
		}},
		// Kept, as a building's frame keeps its tint.
		{"a stopped building's tint", func() { kept = blindsTint() }},
		{"a dimmed building's tint", func() { kept = dimmed() }},
	} {
		t.Run("when "+c.name+" is worked out", func(t *testing.T) {
			t.Run("it should allocate nothing", func(t *testing.T) {
				assert.Zero(t, testing.AllocsPerRun(20, c.run))
			})
		})
	}
}
