package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/measure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 141. A line longer than the cap stops a whole read where it
// starts, so the reader must stop there too, whatever comes after.
func TestTranscriptReaderLineCap(t *testing.T) {
	saved := maxTranscriptLine
	maxTranscriptLine = 16 << 10
	t.Cleanup(func() { maxTranscriptLine = saved })

	prompt := func(ts string) string {
		return `{"type":"user","sessionId":"s","cwd":"/p","timestamp":"` + ts + `","message":{"role":"user","content":"go"}}` + "\n"
	}
	long := `{"type":"user","sessionId":"s","timestamp":"2026-09-16T19:50:05.000Z","message":{"role":"user","content":"` +
		strings.Repeat("x", 32<<10) + `"}}` + "\n"
	appendTo := func(t *testing.T, path, data string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		require.NoError(t, err)
		_, err = f.WriteString(data)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	t.Run("when a line longer than the cap was read", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "s.jsonl")
		appendTo(t, path, prompt("2026-09-16T19:50:00.000Z")+long)
		reader := NewTranscriptReader(path)
		_, err := reader.Read()
		require.NoError(t, err)

		t.Run("and more is appended", func(t *testing.T) {
			appended := prompt("2026-09-16T19:51:00.000Z")
			appendTo(t, path, appended)
			before := measure.Read(measure.BytesRead)
			got, err := reader.Read()
			require.NoError(t, err)
			read := measure.Read(measure.BytesRead) - before
			whole, err := ReadTranscript(path)
			require.NoError(t, err)

			t.Run("it should derive what a whole read derives", func(t *testing.T) {
				assert.Equal(t, whole, got)
			})

			t.Run("it should not read the long line again", func(t *testing.T) {
				assert.LessOrEqual(t, read, int64(len(appended)+headCheckBytes+1))
			})
		})

		t.Run("and the file is then cut back to before the long line", func(t *testing.T) {
			require.NoError(t, os.Truncate(path, int64(len(prompt("2026-09-16T19:50:00.000Z")))+20))
			got, err := reader.Read()
			require.NoError(t, err)
			whole, err := ReadTranscript(path)
			require.NoError(t, err)

			t.Run("it should derive what a whole read derives", func(t *testing.T) {
				assert.Equal(t, whole, got)
			})
		})
	})

	t.Run("when a line longer than the cap arrives in pieces", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "s.jsonl")
		appendTo(t, path, prompt("2026-09-16T19:50:00.000Z"))
		reader := NewTranscriptReader(path)
		var got Transcript
		for _, piece := range []string{long[:10000], long[10000:20000], long[20000:], prompt("2026-09-16T19:51:00.000Z")} {
			appendTo(t, path, piece)
			var err error
			got, err = reader.Read()
			require.NoError(t, err)
		}
		whole, err := ReadTranscript(path)
		require.NoError(t, err)

		t.Run("it should derive what a whole read derives", func(t *testing.T) {
			assert.Equal(t, whole, got)
		})
	})
}
