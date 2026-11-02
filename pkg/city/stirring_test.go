package city

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

// Item 212: a building is quiet while none of its session's timed states
// runs, and the renderer asks which sessions have one once a frame. A
// state left out of Stirring would leave a building drawn as it was
// while it moved; each state is a case here, and StirringStates names
// them, so the renderer's guard can hold its reads to them.
func TestStirring(t *testing.T) {
	at := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	scene := func() *Scene {
		s := NewScene(NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		return s
	}
	const id = "s1"
	states := map[string]func(s *Scene){
		"arriving":    func(s *Scene) { s.voyages = append(s.voyages, &Voyage{SessionID: id, Kind: Arrival, StartedAt: at}) },
		"hauled":      func(s *Scene) { s.hauls = append(s.hauls, Haul{SessionID: id, Since: at, For: time.Minute}) },
		"settling":    func(s *Scene) { s.settling = map[string]settling{id: {since: at}} },
		"celebrating": func(s *Scene) { s.celebrating = map[string]time.Time{id: at} },
		"pouring":     func(s *Scene) { s.jobs = map[string]compactionJob{id: {since: at}} },
		"loads":       func(s *Scene) { s.loads = map[string][]load{id: {{since: at}}} },
		"hoisting":    func(s *Scene) { s.hoists = map[string]hoist{id + " https://example.com/pr/1": {since: at}} },
		"struck":      func(s *Scene) { s.struck = map[string]time.Time{id: at} },
	}

	t.Run("when the states are named", func(t *testing.T) {
		t.Run("it should have a case for every one of them", func(t *testing.T) {
			var cased []string
			for name := range states {
				cased = append(cased, name)
			}
			assert.ElementsMatch(t, StirringStates, cased)
		})
	})

	for name, start := range states {
		t.Run("when a session is "+name, func(t *testing.T) {
			s := scene()
			start(s)

			t.Run("it should be stirring", func(t *testing.T) {
				assert.True(t, s.Stirring(nil)[id])
			})
		})
	}

	t.Run("when a session has none of them", func(t *testing.T) {
		t.Run("it should not be stirring", func(t *testing.T) {
			assert.False(t, scene().Stirring(nil)[id])
		})
	})

	// Item 294: a state that has run out is let go of on the scene's tick,
	// and never by a read, so every read of one frame sees the scene at
	// one instant, and the next frame sees what this one did.
	long := at.Add(-time.Hour)
	building := &Building{Session: state.Session{ID: id, State: state.NeedsYou}}
	ranOut := map[string]struct {
		start func(s *Scene)
		read  func(s *Scene)
	}{
		"arriving": {
			func(s *Scene) { s.voyages = append(s.voyages, &Voyage{SessionID: id, Kind: Arrival, StartedAt: long}) },
			func(s *Scene) { s.Voyages() },
		},
		"hauled": {
			func(s *Scene) { s.hauls = append(s.hauls, Haul{SessionID: id, Since: long, For: time.Minute}) },
			func(s *Scene) { s.Hauls() },
		},
		"settling": {
			func(s *Scene) { s.settling = map[string]settling{id: {since: long}} },
			func(s *Scene) { s.Settling(id) },
		},
		"celebrating": {
			func(s *Scene) { s.celebrating = map[string]time.Time{id: long} },
			func(s *Scene) { s.Celebration(id) },
		},
		"pouring": {
			func(s *Scene) { s.jobs = map[string]compactionJob{id: {since: long}} },
			func(s *Scene) { s.CompactionJob(id) },
		},
		"loads": {
			func(s *Scene) { s.loads = map[string][]load{id: {{since: long}}} },
			func(s *Scene) { s.Loads(id) },
		},
		"hoisting": {
			func(s *Scene) { s.hoists = map[string]hoist{id + " https://example.com/pr/1": {since: long}} },
			func(s *Scene) { s.Animating() },
		},
		"struck": {
			func(s *Scene) { s.struck = map[string]time.Time{id: long} },
			func(s *Scene) { s.Scaffold(building) },
		},
	}

	t.Run("when the states that run out are named", func(t *testing.T) {
		t.Run("it should have a case for every one of them", func(t *testing.T) {
			var cased []string
			for name := range ranOut {
				cased = append(cased, name)
			}
			assert.ElementsMatch(t, StirringStates, cased)
		})
	})

	for name, c := range ranOut {
		t.Run("when a session's "+name+" has run out", func(t *testing.T) {
			t.Run("and the scene has ticked", func(t *testing.T) {
				s := scene()
				c.start(s)
				s.Animate(1.0 / 30)

				t.Run("it should not be stirring", func(t *testing.T) {
					assert.False(t, s.Stirring(nil)[id])
				})
			})

			t.Run("and two frames read it before the scene ticks", func(t *testing.T) {
				s := scene()
				c.start(s)
				before := s.Stirring(nil)[id]
				c.read(s)
				c.read(s)

				t.Run("it should be as stirring as before they read it", func(t *testing.T) {
					assert.Equal(t, before, s.Stirring(nil)[id])
				})
			})
		})
	}

	t.Run("when its flag went up long ago", func(t *testing.T) {
		s := scene()
		s.hoists = map[string]hoist{id + " https://example.com/pr/1": {since: at.Add(-time.Hour)}}

		t.Run("it should not be stirring", func(t *testing.T) {
			assert.False(t, s.Stirring(nil)[id])
		})
	})
}
