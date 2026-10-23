package state_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// pathStats is a statistics store that ingests by path, and notes each
// ingest it was asked for.
type pathStats struct {
	mu      sync.Mutex
	ingests []pathIngest
}

type pathIngest struct {
	full   bool
	paths  []string
	listed bool
}

func (p *pathStats) Ingest(string, time.Time, func()) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ingests = append(p.ingests, pathIngest{full: true})
	return nil
}

func (p *pathStats) IngestPaths(_ string, paths []string, listed bool, _ time.Time, _ func()) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ingests = append(p.ingests, pathIngest{paths: paths, listed: listed})
	return nil
}

func (p *pathStats) Hefts() (map[string]claude.Usage, error) { return map[string]claude.Usage{}, nil }

func (p *pathStats) MovedHefts() (map[string]claude.Usage, []string, error) {
	return map[string]claude.Usage{}, nil, nil
}

func (p *pathStats) since(n int) []pathIngest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]pathIngest(nil), p.ingests[n:]...)
}

func (p *pathStats) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.ingests)
}

// Item 205: a stats pass ingested every transcript, and read every row,
// on every rescan. The loader now hands it the files it saw change, and
// a pass that ingests everything runs on the first and every 10 minutes.
func TestStatsPassIngestsWhatChanged(t *testing.T) {
	const cwd = "/w/stats-paths"
	stamp := func(at time.Time) string { return at.Format("2006-01-02T15:04:05.000Z") }
	start := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	id := func(i int) string { return fmt.Sprintf("00000000-0000-4205-b000-%012d", i) }
	type fixture struct {
		home   *helpers.Home
		loader *state.Loader
		src    *pathStats
		clock  time.Time
		paths  []string
		loads  int
	}
	idle := func(t *testing.T, f *fixture) {
		t.Helper()
		require.Eventually(t, func() bool { return !state.StatsPassRunning(f.loader) }, 5*time.Second, 5*time.Millisecond)
	}
	load := func(t *testing.T, f *fixture) {
		t.Helper()
		f.loads++
		_, err := f.loader.Load(start.Add(time.Duration(f.loads) * time.Second))
		require.NoError(t, err)
		idle(t, f)
	}
	setup := func(t *testing.T) *fixture {
		t.Helper()
		f := &fixture{home: helpers.NewHome(t), src: &pathStats{}, clock: start}
		for i := 0; i < 5; i++ {
			f.paths = append(f.paths, f.home.Transcript(id(i), cwd, helpers.UserPrompt(id(i), cwd, stamp(start)), helpers.AssistantReply(id(i), fmt.Sprintf("msg_%d", i), stamp(start))))
		}
		f.loader = state.NewLoader(f.home.Path, state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}).WithStats(f.src, nil)
		state.SetStatsClock(f.loader, func() time.Time { return f.clock })
		load(t, f)
		for _, dir := range f.loader.WatchDirs() {
			f.loader.Watching(dir, true)
		}
		load(t, f)
		return f
	}

	t.Run("when the first pass has ingested everything", func(t *testing.T) {
		t.Run("it should have ingested everything", func(t *testing.T) {
			f := setup(t)
			assert.True(t, f.src.since(0)[0].full)
		})

		t.Run("and one stopped transcript changes", func(t *testing.T) {
			f := setup(t)
			before := f.src.count()
			f.home.AppendTranscript(id(2), cwd, helpers.AssistantReply(id(2), "msg_more", stamp(start.Add(time.Minute))))
			f.loader.Changed(f.paths[2])
			load(t, f)
			ingests := f.src.since(before)

			t.Run("it should ingest only that transcript", func(t *testing.T) {
				assert.Equal(t, []pathIngest{{paths: []string{f.paths[2]}}}, ingests)
			})
		})

		t.Run("and a stopped session's subagent writes as the session does", func(t *testing.T) {
			f := setup(t)
			before := f.src.count()
			sub := f.home.Subagent(id(4), cwd, "a205", "", helpers.UserPrompt(id(4), cwd, stamp(start.Add(time.Minute))))
			f.home.AppendTranscript(id(4), cwd, helpers.AssistantReply(id(4), "msg_sub", stamp(start.Add(time.Minute))))
			f.loader.Changed(f.paths[4])
			load(t, f)

			t.Run("it should ingest the transcript and the subagent's", func(t *testing.T) {
				assert.Equal(t, []pathIngest{{paths: []string{f.paths[4], sub}}}, f.src.since(before))
			})
		})

		t.Run("and a live session's transcript grows", func(t *testing.T) {
			f := setup(t)
			f.home.Session(4205, id(1), cwd, "interactive", "busy")
			load(t, f)
			before := f.src.count()
			f.home.AppendTranscript(id(1), cwd, helpers.AssistantReply(id(1), "msg_live", stamp(start.Add(time.Minute))))
			f.loader.Changed(f.paths[1])
			load(t, f)

			t.Run("it should ingest only that transcript", func(t *testing.T) {
				assert.Equal(t, []pathIngest{{paths: []string{f.paths[1]}}}, f.src.since(before))
			})
		})

		t.Run("and a transcript is deleted", func(t *testing.T) {
			f := setup(t)
			before := f.src.count()
			require.NoError(t, os.Remove(f.paths[3]))
			f.loader.Changed(f.paths[3])
			load(t, f)
			ingests := f.src.since(before)

			t.Run("it should look for transcripts that are gone", func(t *testing.T) {
				require.Len(t, ingests, 1)
				assert.True(t, ingests[0].listed)
			})
		})

		t.Run("and nothing changes", func(t *testing.T) {
			f := setup(t)
			before := f.src.count()
			load(t, f)

			t.Run("it should start no pass", func(t *testing.T) {
				assert.Empty(t, f.src.since(before))
			})
		})

		t.Run("and ten minutes pass", func(t *testing.T) {
			f := setup(t)
			before := f.src.count()
			f.clock = f.clock.Add(10 * time.Minute)
			load(t, f)

			t.Run("it should ingest everything", func(t *testing.T) {
				assert.Equal(t, []pathIngest{{full: true}}, f.src.since(before))
			})
		})
	})
}
