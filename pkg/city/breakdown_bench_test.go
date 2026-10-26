package city_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

// realHistoryAt is the moment the real-sized history ends.
var realHistoryAt = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// realHistory is a statistics store the real history's size: 31,600
// messages over 7 days, about 4,500 in the last day, in 200 sessions
// across 150 projects, one message in 20 also held by a fork. It is
// written by this build, so it has the change log.
func realHistory(b *testing.B) string {
	b.Helper()
	const (
		messages = 31_600
		sessions = 200
		projects = 150
	)
	path := filepath.Join(b.TempDir(), "stats.db")
	s, err := stats.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	tx, err := s.DB().Begin()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	exec := func(query string, args ...any) {
		if _, err := tx.Exec(query, args...); err != nil {
			b.Fatal(err)
		}
	}
	for p := range projects {
		exec("INSERT INTO dirs (id, path) VALUES (?, ?)", p+1, fmt.Sprintf("/home/user/src/project-%03d", p))
	}
	for i := range sessions {
		exec("INSERT INTO sessions (id) VALUES (?)", sessionID(i))
	}
	step := 7 * 24 * time.Hour / messages
	models := []string{"claude-opus-5-5", "claude-opus-5-5", "claude-opus-5-5", "claude-haiku-4-5"}
	for m := range messages {
		session := min(m/(messages/sessions), sessions-1)
		id := fmt.Sprintf("msg_%06d", m)
		at := realHistoryAt.Add(-time.Duration(m+1) * step).UnixMilli()
		exec(`INSERT INTO messages (id, at, model, dir_id, thread, input, output, cache_create, cache_read, cache_create_5m, cache_create_1h)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, at, models[m%len(models)], session%projects+1, sessionID(session), 3+m%40, 200+m%900, 1000+m%5000, 40_000+m%90_000, 0, 1000+m%5000)
		exec("INSERT INTO message_sessions (message_id, session_id) VALUES (?, ?)", id, sessionID(session))
		if m%20 == 0 {
			exec("INSERT INTO message_sessions (message_id, session_id) VALUES (?, ?)", id, sessionID((session+1)%sessions))
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	return path
}

func sessionID(i int) string { return fmt.Sprintf("%08d-0000-4000-8000-000000000215", i) }

// BenchmarkBreakdown is what a city pays for its 24 h breakdown on the
// real history's size of store: read whole, as a new ledger reads it,
// and brought up to date after a new message, as a ledger keeps it
// (item 215).
func BenchmarkBreakdown(b *testing.B) {
	layout := city.NewLayout()

	b.Run("full", func(b *testing.B) {
		path := realHistory(b)
		for b.Loop() {
			ledger := stats.NewLedger(path)
			c := city.BuildWithLedger(state.Snapshot{At: realHistoryAt}, layout, ledger)
			if !c.Plant.Day.Priced {
				b.Fatal(c.Plant.Day.Reason)
			}
			ledger.Close()
		}
	})

	b.Run("incremental", func(b *testing.B) {
		path := realHistory(b)
		writer, err := stats.Open(path)
		if err != nil {
			b.Fatal(err)
		}
		defer func() { _ = writer.Close() }()
		ledger := stats.NewLedger(path)
		defer ledger.Close()
		at := realHistoryAt
		city.BuildWithLedger(state.Snapshot{At: at}, layout, ledger)
		n := 0
		for b.Loop() {
			b.StopTimer()
			n++
			at = at.Add(stats.LedgerCache)
			if _, err := writer.DB().Exec(`INSERT INTO messages (id, at, model, dir_id, thread, input, output, cache_create, cache_read)
				VALUES (?, ?, 'claude-opus-5-5', 1, ?, 3, 200, 0, 40000)`, fmt.Sprintf("msg_bench_%d", n), at.Add(-time.Second).UnixMilli(), sessionID(0)); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			c := city.BuildWithLedger(state.Snapshot{At: at}, layout, ledger)
			if !c.Plant.Day.Priced {
				b.Fatal(c.Plant.Day.Reason)
			}
		}
	})
}

// BenchmarkTiedViews is what the store pays to place 20 sessions, each
// in a temporary directory of its own, at the district they are tied
// to, against none: for the 24 h window, and for the 3-month total of
// that district and of one no session is tied to (item 285).
func BenchmarkTiedViews(b *testing.B) {
	const project = "/home/user/src/project-000"
	path := realHistory(b)
	store, err := stats.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	tied := map[string]string{}
	for i := range 20 {
		id := sessionID(i * 10)
		tied[id] = project
		dir := 1000 + i
		if _, err := store.DB().Exec("INSERT INTO dirs (id, path) VALUES (?, ?)", dir, fmt.Sprintf("/tmp/tmp.bench%02d", i)); err != nil {
			b.Fatal(err)
		}
		if _, err := store.DB().Exec("UPDATE messages SET dir_id = ? WHERE thread = ?", dir, id); err != nil {
			b.Fatal(err)
		}
	}
	days := stats.Last3Months(stats.DateOf(realHistoryAt))
	for _, c := range []struct {
		name string
		tied map[string]string
	}{{"untied", nil}, {"tied", tied}} {
		b.Run("window, "+c.name, func(b *testing.B) {
			for b.Loop() {
				if _, err := store.Window(realHistoryAt.Add(-24*time.Hour), realHistoryAt, 24, city.ProjectRoot, c.tied); err != nil {
					b.Fatal(err)
				}
			}
		})
		for _, d := range []struct{ name, root string }{{"tied district", project}, {"other district", "/home/user/src/project-050"}, {"Temporary", city.TemporaryRoot}} {
			b.Run(d.name+" total, "+c.name, func(b *testing.B) {
				f := stats.Filter{Root: city.ProjectRoot, Tied: c.tied, Only: []string{d.root}}
				for b.Loop() {
					if _, _, err := store.Totals(days, f, time.UTC); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
