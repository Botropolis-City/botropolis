package render

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 207: the CPU of recording the aged lots of a city like the real
// history, most of its projects overgrown, from snapshots that change
// nothing about them, as an unfocused city records them for each. It
// reports and never gates.
func BenchmarkTheAgedLots(b *testing.B) {
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := range 450 {
		sessions = append(sessions, state.Session{
			ID: fmt.Sprintf("s%03d", i), CWD: fmt.Sprintf("/src/project-%03d", i%150),
			State: state.Resting, LastActivity: at.Add(-time.Duration(40+i%30) * 24 * time.Hour),
		})
	}
	snapshot := state.Snapshot{At: at, Sessions: sessions}
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return at }, time.UTC)
	scene.Resize(1600, 900)
	scene.SetSnapshot(snapshot)
	cities := []*city.City{city.Build(snapshot, city.NewLayout()), city.Build(snapshot, city.NewLayout())}
	cam := scene.Camera()
	g := &Game{scene: scene}
	l := g.recorder()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		l.reset()
		l.begin()
		g.agedLots(l.dry, cam, cities[i%2])
		l.end()
		i++
	}
}
