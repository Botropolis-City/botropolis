package state_test

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// laterTranscript writes a transcript of turns prompt-and-reply pairs a
// second apart from start, padded to about size bytes.
func laterTranscript(t *testing.T, home *helpers.Home, sid string, start time.Time, size int) {
	t.Helper()
	path := home.Transcript(sid, loaderCWD)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	w := bufio.NewWriterSize(f, 1<<20)
	pad := fmt.Sprintf(`"content":"%0*d"`, 2048, 0)
	written := 0
	for turn := 0; written < size; turn++ {
		ts := start.Add(time.Duration(turn) * time.Second).Format(time.RFC3339Nano)
		prompt := helpers.UserPrompt(sid, loaderCWD, ts)
		prompt = prompt[:len(prompt)-len(`"content":"hi"}}`)] + pad + "}}"
		n, err := fmt.Fprintf(w, "%s\n%s\n", prompt, helpers.AssistantReply(sid, fmt.Sprintf("msg_%06d", turn), ts))
		require.NoError(t, err)
		written += n
	}
	require.NoError(t, w.Flush())
	require.NoError(t, f.Close())
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		require.FailNow(t, "the later read never finished")
	}
}

// laneStats is a statistics store that notes, when its ingest starts,
// how many transcripts the later read had read.
type laneStats struct {
	mu        sync.Mutex
	loader    *state.Loader
	sawLater  int
	ingests   int
	ingesting chan struct{}
}

func (s *laneStats) Ingest(string, time.Time, func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ingests == 0 {
		s.sawLater = s.loader.LaterReads()
		close(s.ingesting)
	}
	s.ingests++
	return nil
}

func (s *laneStats) Hefts() (map[string]claude.Usage, error) { return nil, nil }

func (s *laneStats) MovedHefts() (map[string]claude.Usage, []string, error) {
	return nil, nil, nil
}

// Item 203. The daemon's first load read every live transcript whole and
// every transcript active in the plant's window from that window's
// start, unpaced, before the map could draw: 708 MiB of 3.3 GiB on
// Aria's history, 9 s at a core. With a background pass, the first load
// reads every transcript through its window, and those reads follow in
// a paced pass of their own.
func TestLaterReads(t *testing.T) {
	const (
		liveSize  = 2 << 20
		todaySize = 2 << 20
	)
	liveHome := func(t *testing.T) *helpers.Home {
		t.Helper()
		home := helpers.NewHome(t).Session(4242, sidA, loaderCWD, "interactive", "idle")
		laterTranscript(t, home, sidA, now.Add(-2*time.Hour), liveSize)
		return home
	}
	todayHome := func(t *testing.T) *helpers.Home {
		t.Helper()
		home := helpers.NewHome(t)
		laterTranscript(t, home, sidB, now.Add(-20*time.Hour), todaySize)
		return home
	}
	plain := func(t *testing.T, home *helpers.Home, probes state.Probes) state.Snapshot {
		t.Helper()
		snapshot, err := state.NewLoader(home.Path, probes).Load(now)
		require.NoError(t, err)
		return snapshot
	}

	t.Run("when a loader with a background pass loads a live transcript larger than its window", func(t *testing.T) {
		home := liveHome(t)
		done := make(chan struct{}, 8)
		loader := state.NewLoader(home.Path, alive).WithWholeRead(func() { done <- struct{}{} })
		first, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("it should read it through the window", func(t *testing.T) {
			assert.True(t, sessionByID(t, first, sidA).Usage.Partial)
		})

		t.Run("and the later read has finished and the loader loads again", func(t *testing.T) {
			waitDone(t, done)
			again, err := loader.Load(now)
			require.NoError(t, err)

			t.Run("it should hold the session's usage whole", func(t *testing.T) {
				assert.Equal(t, sessionByID(t, plain(t, home, alive), sidA).Usage, sessionByID(t, again, sidA).Usage)
			})

			t.Run("and the transcript grows by a line", func(t *testing.T) {
				line := helpers.AssistantReply(sidA, "msg_appended", now.Format(time.RFC3339Nano))
				home.AppendTranscript(sidA, loaderCWD, line)
				before := measure.Read(measure.BytesRead)
				_, err := loader.Load(now.Add(time.Second))
				require.NoError(t, err)
				read := measure.Read(measure.BytesRead) - before

				t.Run("it should read only what was appended", func(t *testing.T) {
					assert.LessOrEqual(t, read, int64(len(line)+1+8<<10))
				})
			})
		})
	})

	t.Run("when a stopped transcript was active in the plant's window", func(t *testing.T) {
		home := todayHome(t)
		done := make(chan struct{}, 8)
		loader := state.NewLoader(home.Path, dead).WithWholeRead(func() { done <- struct{}{} })
		first, err := loader.Load(now)
		require.NoError(t, err)
		want := plain(t, home, dead).Power

		t.Run("it should leave out of the first load's plant what lies past its window", func(t *testing.T) {
			assert.Less(t, first.Power.Output, want.Output)
		})

		t.Run("and the later read has finished", func(t *testing.T) {
			waitDone(t, done)
			again, err := loader.Load(now)
			require.NoError(t, err)

			t.Run("it should give the plant what a plain load gives", func(t *testing.T) {
				assert.Equal(t, want, again.Power)
			})
		})
	})

	t.Run("when the later read is running and statistics are kept", func(t *testing.T) {
		home := helpers.NewHome(t).Session(4242, sidA, loaderCWD, "interactive", "idle")
		laterTranscript(t, home, sidA, now.Add(-2*time.Hour), 16<<20)
		src := &laneStats{ingesting: make(chan struct{})}
		loader := state.NewLoader(home.Path, alive)
		src.loader = loader
		loader.WithStats(src, func() {})
		_, err := loader.Load(now)
		require.NoError(t, err)
		select {
		case <-src.ingesting:
		case <-time.After(60 * time.Second):
			require.FailNow(t, "the ingest never started")
		}

		t.Run("it should not start the ingest until the later read has ended", func(t *testing.T) {
			src.mu.Lock()
			defer src.mu.Unlock()
			assert.Equal(t, 1, src.sawLater)
		})
	})

	t.Run("when the later read has more than a bar's worth to read", func(t *testing.T) {
		home := helpers.NewHome(t).Session(4242, sidA, loaderCWD, "interactive", "idle")
		laterTranscript(t, home, sidA, now.Add(-2*time.Hour), 40<<20)
		done := make(chan struct{}, 8)
		loader := state.NewLoader(home.Path, alive).WithWholeRead(func() { done <- struct{}{} })
		first, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("it should say how far it has got", func(t *testing.T) {
			assert.NotNil(t, first.Scan)
		})

		t.Run("and it has ended", func(t *testing.T) {
			waitDone(t, done)

			t.Run("it should say nothing", func(t *testing.T) {
				assert.Nil(t, loader.Progress())
			})
		})
	})

	t.Run("when a loader without a background pass loads a live transcript larger than its window", func(t *testing.T) {
		snapshot := plain(t, liveHome(t), alive)

		t.Run("it should read it whole", func(t *testing.T) {
			assert.False(t, sessionByID(t, snapshot, sidA).Usage.Partial)
		})
	})
}
