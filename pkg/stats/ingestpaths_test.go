package stats_test

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/stats"
)

// Item 205: a daemon's pass ingested every transcript, and read every
// row, on every rescan. IngestPaths reads only the files the loader saw
// change, so after every step of a replay it is held to what a store
// that ingests everything holds.
func TestIngestPathsEqualsIngest(t *testing.T) {
	const steps = 120
	type state struct {
		hefts map[string]claude.Usage
		gone  map[string]bool
	}
	for _, seed := range []int64{205, 206, 207} {
		t.Run(fmt.Sprintf("when writes, subagents, truncations and deletions are replayed for %d steps, with seed %d", steps, seed), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			f := newFixture(t)
			incremental := open(t, filepath.Join(t.TempDir(), "incremental.db"))
			sessions := []string{}
			paths := map[string]string{}
			at := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
			n := 0
			line := func(session string) string {
				n++
				at = at.Add(time.Minute)
				return reply{session: session, id: fmt.Sprintf("msg_%d", n), at: at.Format(time.RFC3339Nano), cwd: lib,
					usage: usage{Input: int64(r.Intn(100)), Output: int64(r.Intn(100)), CacheRead: int64(r.Intn(100))}}.line()
			}
			snapshot := func(s *stats.Store) state {
				hefts, err := s.Hefts()
				require.NoError(t, err)
				gone := map[string]bool{}
				for _, session := range sessions {
					var at sql.NullInt64
					_ = s.DB().QueryRow("SELECT transcript_gone_at FROM sessions WHERE id = ?", session).Scan(&at)
					gone[session] = at.Valid
				}
				return state{hefts: hefts, gone: gone}
			}
			for step := 0; step < steps; step++ {
				var changed []string
				listed := false
				switch k := r.Intn(6); {
				case k == 0 || len(sessions) == 0:
					session := fmt.Sprintf("00000000-0000-4205-a000-%012d", len(sessions))
					sessions = append(sessions, session)
					paths[session] = f.transcript(session, line(session))
					changed, listed = []string{paths[session]}, true
				case k == 1 || k == 2:
					session := sessions[r.Intn(len(sessions))]
					if _, err := os.Stat(paths[session]); err == nil {
						f.appendRaw(paths[session], line(session)+"\n")
						changed = []string{paths[session]}
					}
				case k == 3:
					session := sessions[r.Intn(len(sessions))]
					changed = []string{f.subagent(session, fmt.Sprintf("a%d", r.Intn(3)), line(session))}
				case k == 4:
					session := sessions[r.Intn(len(sessions))]
					if _, err := os.Stat(paths[session]); err == nil {
						f.write(paths[session], line(session))
						changed = []string{paths[session]}
					}
				default:
					session := sessions[r.Intn(len(sessions))]
					_ = os.Remove(paths[session])
					changed, listed = []string{paths[session]}, true
				}
				require.NoError(t, f.store.Ingest(f.projects, f.now, nil))
				require.NoError(t, incremental.IngestPaths(f.projects, changed, listed, f.now, nil))
				if !assert.Equal(t, snapshot(f.store), snapshot(incremental), "step %d", step) {
					return
				}
			}
		})
	}
}
