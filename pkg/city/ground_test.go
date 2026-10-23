package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 206: the ground's generation moves when what the standing layer
// draws from the city moves (item 207's contract), and holds still for a
// snapshot that moves only what is drawn above it.
func TestTheGroundsGeneration(t *testing.T) {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	// shown is a scene showing a city the size of the real history,
	// then what act does to it, and whether the ground's generation
	// moved for that.
	shown := func(t *testing.T, act func(*city.Scene)) (*city.Scene, bool) {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(1600, 900)
		snap := realSized(at)
		snap.Servers = []state.Server{{Name: "files", Configured: true, Calls: 3, Sessions: 1}}
		snap.Sessions[0].MCPCalls = map[string]int{"files": 3}
		snap.Teams = []claude.Team{{Name: "crew", LeadSessionID: snap.Sessions[0].ID}}
		snap.Sessions[1].Team = "crew"
		s.SetSnapshot(snap)
		before := s.GroundGeneration()
		act(s)
		return s, s.GroundGeneration() != before
	}
	// next is a scene's snapshot again, changed.
	next := func(change func(*state.Snapshot)) func(*city.Scene) {
		return func(s *city.Scene) {
			snap := s.Snapshot()
			snap.Sessions = append([]state.Session(nil), snap.Sessions...)
			snap.Roads = append([]state.Road(nil), snap.Roads...)
			change(&snap)
			s.SetSnapshot(snap)
		}
	}

	for _, c := range []struct {
		scenario string
		act      func(*city.Scene)
		moves    bool
	}{
		{"the next snapshot moves only activity times", next(func(snap *state.Snapshot) {
			for i := range snap.Sessions {
				snap.Sessions[i].LastActivity = snap.Sessions[i].LastActivity.Add(time.Second)
			}
		}), false},
		{"the next snapshot changes only one session's state", next(func(snap *state.Snapshot) {
			snap.Sessions[4].State = state.Waiting
		}), false},
		{"a session's MCP calls tick up", next(func(snap *state.Snapshot) {
			snap.Sessions[0].MCPCalls = map[string]int{"files": 4}
		}), true},
		{"a road's messages grow", next(func(snap *state.Snapshot) {
			snap.Roads[0].Messages++
		}), true},
		{"a road joins two projects", next(func(snap *state.Snapshot) {
			snap.Roads = append(snap.Roads, state.Road{From: "/home/user/src/project-001", To: "/home/user/src/project-002", Messages: 1, Sessions: 1})
		}), true},
		{"a session joins a new project", next(func(snap *state.Snapshot) {
			joined := snap.Sessions[1]
			joined.ID, joined.CWD = "joined-0000-4000-8000-000000000000", "/home/user/src/a-new-project"
			snap.Sessions = append(snap.Sessions, joined)
		}), true},
		{"a team gains a member", next(func(snap *state.Snapshot) {
			snap.Sessions[2].Team = "crew"
		}), true},
		{"the servers change", next(func(snap *state.Snapshot) {
			snap.Servers = append(append([]state.Server(nil), snap.Servers...), state.Server{Name: "search", Configured: true})
		}), true},
		{"the camera turns", func(s *city.Scene) { s.Turn() }, true},
		{"the detail changes", func(s *city.Scene) { s.SetDetail(city.DetailPlain) }, true},
	} {
		t.Run("when a city the size of the real history is shown, and "+c.scenario, func(t *testing.T) {
			s, moved := shown(t, c.act)

			if c.moves {
				t.Run("it should move the ground's generation", func(t *testing.T) {
					assert.True(t, moved)
				})
			} else {
				t.Run("it should not move the ground's generation", func(t *testing.T) {
					assert.False(t, moved)
				})
			}

			t.Run("it should hold the ground of the city built from scratch", func(t *testing.T) {
				assert.Equal(t, city.GroundOf(s.FromScratch()), s.Ground())
			})
		})
	}
}
