package demo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Claude Code can stamp a session's first prompt a moment after a line
// that follows it, and once the scrub dropped the queue line stamped
// before both, the prompt no longer led the file at the earliest moment
// (item 105).
func TestOpenAt(t *testing.T) {
	t.Run("when a transcript's first prompt is stamped a moment after a line that follows it", func(t *testing.T) {
		lines := [][]byte{
			[]byte(line("user", "2026-09-01T10:00:00.865Z")),
			[]byte(line("system", "2026-09-01T10:00:00.864Z")),
			[]byte(turn("2026-09-01T10:05:00.000Z", 3000, 10000, 200)),
		}
		_, opened, err := openAt(lines, time.Date(2026, 9, 1, 7, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		t.Run("it should still take the prompt as the opening", func(t *testing.T) {
			assert.GreaterOrEqual(t, opened, 1)
		})
	})
}

func TestFillHours(t *testing.T) {
	t.Run("when the opening is empty", func(t *testing.T) {
		lines := [][]byte{
			[]byte(line("user", "2026-09-01T10:00:00.000Z")),
			[]byte(turn("2026-09-01T10:05:00.000Z", 3000, 10000, 200)),
		}
		_, err := fillHours(lines, 0, 45*time.Second, recordedID, cwd)

		t.Run("it should refuse, rather than fill from the zero time", func(t *testing.T) {
			assert.ErrorContains(t, err, "no opening")
		})
	})

	t.Run("when an opening is more than a whole day before the rest", func(t *testing.T) {
		lines := [][]byte{
			[]byte(line("user", "2026-08-31T09:00:00.000Z")),
			[]byte(turn("2026-09-01T10:05:00.000Z", 3000, 10000, 200)),
		}
		_, err := fillHours(lines, 1, 45*time.Second, recordedID, cwd)

		t.Run("it should refuse too, as a corrupted timestamp would otherwise fill without bound", func(t *testing.T) {
			assert.ErrorContains(t, err, "more than")
		})
	})
}
