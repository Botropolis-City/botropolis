package city_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// realSized is a snapshot the size of the real history item 197 was
// measured on: 200 sessions in 150 projects, a quarter of them in eight
// groups whose sessions carry the words, commands and pull requests
// group.Order reads.
func realSized(at time.Time) state.Snapshot {
	const (
		sessions = 200
		projects = 150
		groups   = 8
		lines    = 40
		roads    = 80
	)
	snap := state.Snapshot{At: at}
	members := map[string][]string{}
	for i := range sessions {
		root := fmt.Sprintf("/home/user/src/project-%03d", i%projects)
		s := state.Session{
			ID:             fmt.Sprintf("%08d-0000-4000-8000-000000000000", i),
			CWD:            root,
			Title:          fmt.Sprintf("session %d", i),
			State:          []state.State{state.Working, state.Resting, state.NeedsYou, state.Waiting}[i%4],
			Alive:          i%4 != 3,
			StartedAt:      at.Add(-time.Duration(i) * time.Hour),
			LastActivity:   at.Add(-time.Duration(i) * time.Minute),
			ContextPercent: float64(i % 100),
		}
		if i%4 == 0 {
			g := fmt.Sprintf("g%d", (i/4)%groups)
			s.Group = g
			members[g] = append(members[g], s.ID)
			s.PRs = []claude.PR{{Number: i, Repository: fmt.Sprintf("acme/project-%03d", i%projects), URL: fmt.Sprintf("https://code.example.com/acme/project-%03d/pull/%d", i%projects, i), State: claude.PROpen}}
			for l := range lines {
				text := fmt.Sprintf("line %d of session %d: update the parser, then bump to v1.%d.0 once the tests pass", l, i, l%5)
				command := l%3 == 0
				if command {
					text = fmt.Sprintf("git push github main && git tag v1.%d.%d", l%5, i)
				}
				s.OrderLines = append(s.OrderLines, claude.OrderLine{Text: text, Command: command, At: at.Add(-time.Duration(l) * time.Minute)})
			}
			s.LastPrompt = fmt.Sprintf("wait for project-%03d to release v1.2.0 before you start", (i+4)%projects)
			s.Recap = claude.Recap{Text: fmt.Sprintf("blocked on project-%03d until its pull request merges", (i+4*groups)%projects), At: at}
		}
		snap.Sessions = append(snap.Sessions, s)
	}
	for g := range groups {
		id := fmt.Sprintf("g%d", g)
		snap.Groups = append(snap.Groups, state.Group{ID: id, Name: "group " + id, Short: id, Members: members[id]})
	}
	// Roads between projects, as the real history has and item 206 found
	// this fixture had not: routing them was most of what a snapshot cost.
	for r := range roads {
		from, to := (r*7)%projects, (r*13+5)%projects
		if from == to {
			continue
		}
		snap.Roads = append(snap.Roads, state.Road{
			From: fmt.Sprintf("/home/user/src/project-%03d", from), To: fmt.Sprintf("/home/user/src/project-%03d", to),
			Messages: 1 + r%5, Sessions: 1,
		})
	}
	return snap
}

// Item 197: what a snapshot costs a city the size of the real history,
// laid out from nothing, and when only one session's state changed
// since the last. It reports and never gates.
func BenchmarkSetSnapshot(b *testing.B) {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	b.Run("first", func(b *testing.B) {
		snap := realSized(at)
		b.ReportAllocs()
		for b.Loop() {
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return at }, time.UTC)
			s.Resize(1600, 900)
			s.SetSnapshot(snap)
		}
	})
	b.Run("one-state-changed", func(b *testing.B) {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(1600, 900)
		snaps := [2]state.Snapshot{realSized(at), realSized(at)}
		snaps[1].Sessions[1].State = state.NeedsYou
		s.SetSnapshot(snaps[0])
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			i++
			s.SetSnapshot(snaps[i%2])
		}
	})
	// Item 206: what most of a busy city's snapshots are.
	b.Run("activity-only", func(b *testing.B) {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(1600, 900)
		snaps := [2]state.Snapshot{realSized(at), realSized(at)}
		snaps[1].Sessions[0].LastActivity = at.Add(time.Second)
		s.SetSnapshot(snaps[0])
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			i++
			s.SetSnapshot(snaps[i%2])
		}
	})
}
