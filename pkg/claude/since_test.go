package claude_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/measure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 94. The plant sums a stopped transcript's hours since its window
// opened, so a transcript active in the last day is read from the start
// of that hour to its end, with the head for its title.
func TestReadTranscriptSince(t *testing.T) {
	const window = 4096
	since := time.Date(2026, time.September, 16, 20, 30, 0, 0, time.UTC)
	turns := func(prefix string, from time.Time, n int) []string {
		var lines []string
		for i := range n {
			ts := from.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
			lines = append(lines, prompt(ts),
				assistantWithUsage(usageSpec{id: fmt.Sprintf("%s_%03d", prefix, i), ts: ts, in: 10, out: 20}))
		}
		return lines
	}
	head := []string{userLine, modelLine, `{"type":"ai-title","aiTitle":"The head's title","sessionId":"` + sid + `"}`}
	straddling := func(extra ...string) []string {
		lines := append([]string{}, head...)
		lines = append(lines, turns("early", since.Add(-6*time.Hour), 300)...)
		lines = append(lines, extra...)
		lines = append(lines, turns("hour", since.Add(-30*time.Minute), 30)...)
		return append(lines, turns("late", since, 100)...)
	}
	fromHour := func(hourly map[int64]claude.Usage) map[int64]claude.Usage {
		out := map[int64]claude.Usage{}
		for h, u := range hourly {
			if h >= since.Unix()/3600 {
				out[h] = u
			}
		}
		return out
	}
	read := func(t *testing.T, lines []string) (whole, part claude.Transcript) {
		t.Helper()
		path := writeTranscript(t, lines...)
		whole, err := claude.ReadTranscript(path)
		require.NoError(t, err)
		part, err = claude.ReadTranscriptSince(path, window, since)
		require.NoError(t, err)
		return whole, part
	}

	t.Run("when the file is larger than twice the window", func(t *testing.T) {
		t.Run("and its entries straddle the window's opening", func(t *testing.T) {
			whole, part := read(t, append(straddling(), costStateLine))

			t.Run("it should give the same hourly usage from since's hour on as a whole read", func(t *testing.T) {
				assert.Equal(t, fromHour(whole.Hourly), fromHour(part.Hourly))
			})

			t.Run("it should take the title from the head", func(t *testing.T) {
				assert.Equal(t, "The head's title", part.Title)
			})

			t.Run("it should leave out usage from before since's hour", func(t *testing.T) {
				assert.Less(t, part.Usage.Input, whole.Usage.Input)
			})

			t.Run("it should mark its usage partial", func(t *testing.T) {
				assert.True(t, part.Usage.Partial)
			})

			t.Run("and one message's lines straddle the hour's start", func(t *testing.T) {
				hourStart := since.Truncate(time.Hour)
				first := usageSpec{id: "msg_split", ts: hourStart.Add(-time.Second).Format(time.RFC3339Nano), in: 7, out: 9}
				second := first
				second.ts = hourStart.Add(time.Second).Format(time.RFC3339Nano)
				whole, part := read(t, append(straddling(assistantWithUsage(first), assistantWithUsage(second)), costStateLine))

				t.Run("it should bucket the message where a whole read does", func(t *testing.T) {
					assert.Equal(t, fromHour(whole.Hourly), fromHour(part.Hourly))
				})
			})

			t.Run("and its last cost-state record is before the window opens", func(t *testing.T) {
				lines := straddling(append([]string{costStateLine}, turns("between", since.Add(-90*time.Minute), 10)...)...)
				whole, part := read(t, lines)

				t.Run("it should carry the cost a whole read carries", func(t *testing.T) {
					assert.Equal(t, whole.Cost, part.Cost)
				})

				t.Run("it should stop the walk at the record", func(t *testing.T) {
					path := writeTranscript(t, lines...)
					info, err := os.Stat(path)
					require.NoError(t, err)
					before := measure.Take()
					_, err = claude.ReadTranscriptSince(path, window, since)
					require.NoError(t, err)
					assert.Less(t, measure.Take().Since(before)[measure.BytesRead], info.Size())
				})
			})
		})

		t.Run("and its whole content is inside the window", func(t *testing.T) {
			lines := append([]string{head[2]}, turns("late", since, 200)...)
			whole, part := read(t, append(lines, costStateLine))

			t.Run("it should equal a whole read", func(t *testing.T) {
				assert.Equal(t, whole, part)
			})

			t.Run("it should read the file once", func(t *testing.T) {
				path := writeTranscript(t, append(lines, costStateLine)...)
				info, err := os.Stat(path)
				require.NoError(t, err)
				before := measure.Take()
				_, err = claude.ReadTranscriptSince(path, window, since)
				require.NoError(t, err)
				assert.Equal(t, info.Size(), measure.Take().Since(before)[measure.BytesRead])
			})
		})
	})

	t.Run("when the file is no larger than twice the window", func(t *testing.T) {
		lines := append(append([]string{}, head...), turns("late", since, 3)...)
		path := writeTranscript(t, lines...)
		whole, err := claude.ReadTranscript(path)
		require.NoError(t, err)
		part, err := claude.ReadTranscriptSince(path, 1<<20, since)
		require.NoError(t, err)

		t.Run("it should equal a whole read", func(t *testing.T) {
			assert.Equal(t, whole, part)
		})
	})
}
