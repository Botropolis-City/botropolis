package stats_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

// Item 159: a quit part-way through the first launch read every
// transcript it had started from the start again, and a long one is
// hundreds of megabytes.
func TestIngestResumes(t *testing.T) {
	const (
		splitID   = "msg_split"
		stopAfter = stats.CheckpointEvery/stats.YieldEvery + 4
	)
	padding := strings.Replace(prompt(sessionA, lib, "2026-10-18T08:00:00.000Z"), `"hi"`, `"`+strings.Repeat("x", 64<<10)+`"`, 1)
	replyAt := func(id string, output int64) string {
		return reply{session: sessionA, id: id, at: "2026-10-18T08:00:01.000Z", cwd: lib, usage: usage{Output: output}}.line()
	}

	// A transcript of two and a half checkpoints, with a reply every
	// megabyte or so, and one reply written before the first checkpoint
	// and again after the stop, the way a streamed reply is.
	lines := []string{prompt(sessionA, lib, "2026-10-18T07:59:00.000Z"), replyAt(splitID, 1)}
	replies, size := 1, 0
	padTo := func(limit int) {
		for size < limit {
			lines = append(lines, padding)
			size += len(padding) + 1
			if len(lines)%16 == 0 {
				lines = append(lines, replyAt(fmt.Sprintf("msg_%d", replies), int64(replies)))
				replies++
			}
		}
	}
	padTo(stats.CheckpointEvery * 3 / 2)
	lines = append(lines, replyAt(splitID, 9))
	padTo(stats.CheckpointEvery * 5 / 2)

	ingestCounting := func(t *testing.T, f *fixture, ctx context.Context, cancelAt int) (int, error) {
		t.Helper()
		yields := 0
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		err := f.store.IngestContext(cancelCtx, f.projects, f.now, func() {
			if yields++; yields == cancelAt {
				cancel()
			}
		})
		return yields, err
	}

	whole := newFixture(t)
	whole.transcript(sessionA, lines...)
	wholeYields, err := ingestCounting(t, whole, context.Background(), 0)
	require.NoError(t, err)

	t.Run("when ingest stops part-way through a transcript longer than a checkpoint", func(t *testing.T) {
		f := newFixture(t)
		path := f.transcript(sessionA, lines...)
		info, err := os.Stat(path)
		require.NoError(t, err)
		_, stopped := ingestCounting(t, f, context.Background(), stopAfter)
		readTo := f.number("SELECT read_to FROM transcripts WHERE path = ?", path)

		t.Run("it should say it was stopped", func(t *testing.T) {
			assert.ErrorIs(t, stopped, context.Canceled)
		})

		t.Run("it should keep what it read up to the last checkpoint", func(t *testing.T) {
			assert.GreaterOrEqual(t, readTo, int64(stats.CheckpointEvery))
		})

		t.Run("it should not claim the rest of the transcript", func(t *testing.T) {
			assert.Less(t, readTo, info.Size())
		})

		t.Run("and ingest runs again", func(t *testing.T) {
			again, err := ingestCounting(t, f, context.Background(), 0)
			require.NoError(t, err)

			t.Run("it should read on from the checkpoint, not from the start", func(t *testing.T) {
				assert.LessOrEqual(t, again, wholeYields-stats.CheckpointEvery/stats.YieldEvery+1)
			})

			t.Run("it should hold every message once", func(t *testing.T) {
				assert.Equal(t, replies, f.messages())
			})

			t.Run("and the stop landed inside a reply written before and after a checkpoint", func(t *testing.T) {
				t.Run("it should hold that reply once, as it was last written", func(t *testing.T) {
					assert.Equal(t, int64(9), f.number("SELECT output FROM messages WHERE id = ?", splitID))
				})
			})

			t.Run("it should hold the same totals as one uninterrupted ingest", func(t *testing.T) {
				assert.Equal(t, whole.hefts(), f.hefts())
			})
		})
	})
}
