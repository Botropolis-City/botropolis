package proto_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Item 201: the daemon sent its whole snapshot on every update, and
// decoding it was most of what a watched city cost. It now sends what
// changed since what it last sent on the same connection.
func TestDeltas(t *testing.T) {
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	city := func(n int) state.Snapshot {
		s := state.Snapshot{At: at, Power: state.Power{Since: at, Fresh: 10}}
		for g := range 3 {
			s.Groups = append(s.Groups, state.Group{
				ID:      fmt.Sprintf("g-%d", g),
				Name:    fmt.Sprintf("Group %d", g),
				Members: []string{"a", "b"},
				Journal: state.Journal{Active: at},
			})
		}
		for i := range n {
			s.Sessions = append(s.Sessions, state.Session{
				ID:           fmt.Sprintf("%08d-0000-4000-8000-000000000201", i),
				Title:        fmt.Sprintf("Session %d", i),
				CWD:          fmt.Sprintf("/home/user/src/project-%03d", i%150),
				Model:        "claude-opus-5-5",
				Kind:         claude.KindInteractive,
				State:        state.Resting,
				Tool:         "Bash",
				Messages:     map[string]int{"user": i, "assistant": i},
				Usage:        claude.Usage{Input: int64(i) * 1000},
				StartedAt:    at.Add(-time.Duration(i) * time.Minute),
				LastActivity: at.Add(-time.Duration(i) * time.Second),
			})
		}
		return s
	}
	wire := func(t *testing.T, r proto.Response) (proto.Response, int) {
		t.Helper()
		var buf bytes.Buffer
		require.NoError(t, proto.Write(&buf, r))
		n := buf.Len()
		var got proto.Response
		require.NoError(t, proto.NewDecoder(&buf).Decode(&got))
		return got, n
	}
	whole := func(t *testing.T, s state.Snapshot) state.Snapshot {
		t.Helper()
		got, _ := wire(t, proto.Response{Snapshot: &s})
		return *got.Snapshot
	}
	edit := func(s state.Snapshot, f func(*state.Snapshot)) state.Snapshot {
		out := s
		out.Sessions = append([]state.Session(nil), s.Sessions...)
		f(&out)
		return out
	}

	t.Run("when a client is fed deltas through a replay of snapshots", func(t *testing.T) {
		start := city(5)
		steps := []struct {
			name string
			f    func(*state.Snapshot)
		}{
			{"a session's state changes", func(s *state.Snapshot) { s.Sessions[1].State = state.Working }},
			{"a field goes to its zero value", func(s *state.Snapshot) { s.Sessions[1].Tool = "" }},
			{"a nested map changes", func(s *state.Snapshot) { s.Sessions[2].Messages = map[string]int{"user": 9} }},
			{"a nested map goes", func(s *state.Snapshot) { s.Sessions[2].Messages = nil }},
			{"a session joins", func(s *state.Snapshot) {
				s.Sessions = append(s.Sessions, state.Session{ID: "joined", Title: "New", StartedAt: at})
			}},
			{"a session joins in the middle", func(s *state.Snapshot) {
				s.Sessions = append(s.Sessions[:2:2], append([]state.Session{{ID: "middle", Title: "Mid"}}, s.Sessions[2:]...)...)
			}},
			{"a session leaves", func(s *state.Snapshot) { s.Sessions = append(s.Sessions[:0:0], s.Sessions[1:]...) }},
			{"the sessions reorder", func(s *state.Snapshot) {
				s.Sessions[0], s.Sessions[len(s.Sessions)-1] = s.Sessions[len(s.Sessions)-1], s.Sessions[0]
			}},
			{"the power changes", func(s *state.Snapshot) { s.Power.Fresh = 99 }},
			{"a scan appears", func(s *state.Snapshot) { s.Scan = &state.ScanProgress{Done: 1, Known: 10} }},
			{"the scan moves", func(s *state.Snapshot) { s.Scan = &state.ScanProgress{Done: 5, Known: 10} }},
			{"the scan ends", func(s *state.Snapshot) { s.Scan = nil }},
			{"the time moves", func(s *state.Snapshot) { s.At = s.At.Add(time.Second) }},
			{"a group's journal moves", func(s *state.Snapshot) {
				s.Groups = append([]state.Group(nil), s.Groups...)
				s.Groups[1].Journal.Active = at.Add(time.Minute)
			}},
			{"a group forms", func(s *state.Snapshot) {
				s.Groups = append(append([]state.Group(nil), s.Groups...), state.Group{ID: "g-new", Name: "New"})
			}},
			{"a group dissolves", func(s *state.Snapshot) { s.Groups = append([]state.Group(nil), s.Groups[1:]...) }},
			{"every group goes", func(s *state.Snapshot) { s.Groups = []state.Group{} }},
			{"all but one session go", func(s *state.Snapshot) { s.Sessions = city(1).Sessions }},
			{"the last session goes, leaving an empty list", func(s *state.Snapshot) { s.Sessions = []state.Session{} }},
			{"a session comes back", func(s *state.Snapshot) { s.Sessions = city(1).Sessions }},
			{"the last session goes, leaving no list", func(s *state.Snapshot) { s.Sessions = nil }},
			{"sessions come back", func(s *state.Snapshot) { s.Sessions = city(3).Sessions }},
			{"groups come back", func(s *state.Snapshot) { s.Groups = city(0).Groups }},
			{"nothing changes", func(*state.Snapshot) {}},
		}
		sender := proto.NewSender()
		var mirror proto.Mirror
		first, err := sender.Next(start)
		require.NoError(t, err)
		got, _ := wire(t, first)
		_, err = mirror.Take(got)
		require.NoError(t, err)
		current := start
		for _, step := range steps {
			current = edit(current, step.f)
			t.Run("and "+step.name, func(t *testing.T) {
				r, err := sender.Next(current)
				require.NoError(t, err)
				got, _ := wire(t, r)
				held, err := mirror.Take(got)
				require.NoError(t, err)

				t.Run("it should hold the same snapshot as a client fed the full snapshot", func(t *testing.T) {
					assert.Equal(t, whole(t, current), held)
				})
			})
		}
	})

	t.Run("when one session's state changes in a city of 200 sessions", func(t *testing.T) {
		before := city(200)
		after := edit(before, func(s *state.Snapshot) { s.Sessions[7].State = state.Working })
		sender := proto.NewSender()
		r, err := sender.Next(before)
		require.NoError(t, err)
		_, full := wire(t, r)
		r, err = sender.Next(after)
		require.NoError(t, err)
		_, delta := wire(t, r)

		t.Run("it should send less than a twentieth of the full snapshot's bytes", func(t *testing.T) {
			assert.Less(t, delta*20, full)
		})
	})

	t.Run("when the delta would be larger than the full snapshot", func(t *testing.T) {
		sender := proto.NewSender()
		_, err := sender.Next(city(50))
		require.NoError(t, err)
		r, err := sender.Next(state.Snapshot{At: at})
		require.NoError(t, err)

		t.Run("it should send the full snapshot", func(t *testing.T) {
			assert.NotNil(t, r.Snapshot)
		})
	})

	t.Run("when the snapshot's session IDs are not unique", func(t *testing.T) {
		sender := proto.NewSender()
		before := city(20)
		_, err := sender.Next(before)
		require.NoError(t, err)
		r, err := sender.Next(edit(before, func(s *state.Snapshot) { s.Sessions[3].ID = s.Sessions[4].ID }))
		require.NoError(t, err)

		t.Run("it should send the full snapshot", func(t *testing.T) {
			assert.NotNil(t, r.Snapshot)
		})
	})

	t.Run("when a delta's base is not the snapshot the client holds", func(t *testing.T) {
		sender := proto.NewSender()
		before := city(20)
		first, err := sender.Next(before)
		require.NoError(t, err)
		_, err = sender.Next(edit(before, func(s *state.Snapshot) { s.Sessions[1].State = state.Working }))
		require.NoError(t, err)
		third, err := sender.Next(edit(before, func(s *state.Snapshot) { s.Sessions[2].State = state.Working }))
		require.NoError(t, err)
		var mirror proto.Mirror
		got, _ := wire(t, first)
		_, err = mirror.Take(got)
		require.NoError(t, err)
		got, _ = wire(t, third)
		_, err = mirror.Take(got)

		t.Run("it should refuse it as needing a resync", func(t *testing.T) {
			assert.ErrorIs(t, err, proto.ErrResync)
		})
	})
}
