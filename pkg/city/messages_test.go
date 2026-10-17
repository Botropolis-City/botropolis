package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: a message between two sessions in different districts sends
// a van along the road between them, or a carriage along their group's
// line when the line is drawn.
func TestMessages(t *testing.T) {
	start := time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)
	member := func(id, cwd, group string) state.Session {
		s := session(id, cwd, state.Working)
		s.SubagentsInFlight, s.Group = 0, group
		return s
	}
	snap := func(messages, back int, group string) state.Snapshot {
		s := snapshot(member("a", cinders, group), member("b", mullet, group))
		s.Roads = []state.Road{{From: cinders, To: mullet, Messages: messages, BackMessages: back, Sessions: 2}}
		if group != "" {
			s.Groups = []state.Group{{ID: group, Short: "G", Members: []string{"a", "b"}}}
		}
		return s
	}
	type scene struct {
		s     *city.Scene
		clock *time.Time
	}
	open := func(t *testing.T, level city.Liveliness, lines bool, before, after state.Snapshot) scene {
		t.Helper()
		clock := start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetReducedMotion(level.CrossFades())
		s.SetGroupLines(lines)
		s.SetSnapshot(before)
		s.SetSnapshot(after)
		return scene{s: s, clock: &clock}
	}
	district := func(sc scene, root string) *city.District {
		for _, d := range sc.s.City().Districts {
			if d.Root == root {
				return d
			}
		}
		return nil
	}

	t.Run("when a session sends one message to a session in another district", func(t *testing.T) {
		sc := open(t, city.Lively, true, snap(0, 0, ""), snap(1, 0, ""))
		vans := sc.s.Messages()

		t.Run("it should send one van from the sender's district to the recipient's", func(t *testing.T) {
			require.Len(t, vans, 1)
			assert.Equal(t, [3]any{cinders, mullet, false}, [3]any{vans[0].From.Root, vans[0].To.Root, vans[0].Train})
		})

		t.Run("its path should be the road's street path, in that direction", func(t *testing.T) {
			require.Len(t, vans, 1)
			require.NotEmpty(t, sc.s.City().Streets)
			assert.Equal(t, sc.s.City().Streets[0].Path, vans[0].Path)
		})

		t.Run("and its trip is over", func(t *testing.T) {
			require.Len(t, vans, 1)
			*sc.clock = start.Add(vans[0].For() + time.Millisecond)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, sc.s.Messages())
			})
		})
	})

	t.Run("when three messages arrive in one snapshot", func(t *testing.T) {
		sc := open(t, city.Lively, true, snap(0, 0, ""), snap(3, 0, ""))
		vans := sc.s.Messages()

		t.Run("it should send three vans, spaced so they do not overlap", func(t *testing.T) {
			require.Len(t, vans, 3)
			assert.Equal(t, [2]time.Duration{city.VanGap, city.VanGap},
				[2]time.Duration{vans[1].Since.Sub(vans[0].Since), vans[2].Since.Sub(vans[1].Since)})
		})
	})

	t.Run("when the reply comes back the other way", func(t *testing.T) {
		sc := open(t, city.Lively, true, snap(1, 0, ""), snap(1, 1, ""))
		vans := sc.s.Messages()

		t.Run("it should send a van the other way", func(t *testing.T) {
			require.Len(t, vans, 1)
			assert.Equal(t, [2]string{mullet, cinders}, [2]string{vans[0].From.Root, vans[0].To.Root})
		})
	})

	t.Run("when both sessions are stations on their group's drawn line", func(t *testing.T) {
		sc := open(t, city.Lively, true, snap(0, 0, "g1"), snap(1, 0, "g1"))
		vans := sc.s.Messages()

		t.Run("it should send a carriage along the line between the two stations, and no van", func(t *testing.T) {
			require.Len(t, vans, 1)
			require.True(t, vans[0].Train)
			path := vans[0].Path
			require.NotEmpty(t, path)
			assert.Equal(t, [2]bool{true, true}, [2]bool{
				district(sc, cinders).Rect.Contains(path[0]),
				district(sc, mullet).Rect.Contains(path[len(path)-1]),
			})
		})

		t.Run("and group lines are turned off", func(t *testing.T) {
			off := open(t, city.Lively, false, snap(0, 0, "g1"), snap(1, 0, "g1"))

			t.Run("it should send a van", func(t *testing.T) {
				require.Len(t, off.s.Messages(), 1)
				assert.False(t, off.s.Messages()[0].Train)
			})
		})
	})

	// Slice 9: a message between two ordinary sessions reaches the road
	// the same way, and sends one van.
	t.Run("when a peer message arrives between two districts", func(t *testing.T) {
		sender := member("a", cinders, "")
		sender.PID = 42
		peered := func(n int) state.Snapshot {
			recipient := member("b", mullet, "")
			for i := 0; i < n; i++ {
				recipient.PeerMessages = append(recipient.PeerMessages, claude.PeerMessage{ID: fmt.Sprint(i), FromPID: 42})
			}
			s := snapshot(sender, recipient)
			s.Roads = state.Roads(s.Sessions, nil)
			return s
		}
		sc := open(t, city.Lively, true, peered(0), peered(1))

		t.Run("it should send one van, as a team message does", func(t *testing.T) {
			vans := sc.s.Messages()
			require.Len(t, vans, 1)
			assert.Equal(t, [2]string{cinders, mullet}, [2]string{vans[0].From.Root, vans[0].To.Root})
		})
	})

	t.Run("when the map first opens on messages already sent", func(t *testing.T) {
		clock := start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetSnapshot(snap(5, 2, ""))

		t.Run("it should send nothing", func(t *testing.T) {
			assert.Empty(t, s.Messages())
		})
	})

	t.Run("when the level is still", func(t *testing.T) {
		sc := open(t, city.Still, true, snap(0, 0, ""), snap(1, 0, ""))
		vans := sc.s.Messages()

		t.Run("it should fade the van in at the recipient's end, with no drive", func(t *testing.T) {
			require.Len(t, vans, 1)
			assert.Equal(t, [2]any{true, vans[0].Path[len(vans[0].Path)-1]}, [2]any{vans[0].Fades, vans[0].At(start.Add(time.Millisecond))})
		})
	})

	t.Run("while a van drives", func(t *testing.T) {
		sc := open(t, city.Lively, true, snap(0, 0, ""), snap(1, 0, ""))
		*sc.clock = start.Add(200 * time.Millisecond)

		t.Run("the scene should be animating", func(t *testing.T) {
			assert.True(t, sc.s.Animating())
		})
	})
}
