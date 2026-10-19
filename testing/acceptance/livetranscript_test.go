package acceptance_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 141. Item 138 measured one load spending 3.7 s of CPU re-reading a
// 339 MB live transcript because the session had written one more line.
// What a load reads when a live session writes a line is what it wrote,
// however long the session has lived.
func TestLiveTranscriptAppend(t *testing.T) {
	const (
		sid        = "14114114-1111-4141-8141-141141141141"
		cwd        = "/home/user/src/long-lived"
		transcript = 16 << 20
	)
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	home := helpers.NewHome(t).Session(14141, sid, cwd, "interactive", "idle")
	path := home.Transcript(sid, cwd, helpers.UserPrompt(sid, cwd, at.Add(-time.Hour).Format(time.RFC3339Nano)))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	for i := 0; ; i++ {
		info, err := f.Stat()
		require.NoError(t, err)
		if info.Size() >= transcript {
			break
		}
		ts := at.Add(-time.Hour + time.Duration(i)*time.Millisecond).Format(time.RFC3339Nano)
		_, err = fmt.Fprintf(f, "%s\n%s\n", helpers.UserPrompt(sid, cwd, ts), helpers.AssistantReply(sid, fmt.Sprintf("msg_%07d", i), ts))
		require.NoError(t, err)
	}
	require.NoError(t, f.Close())
	alive := state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }}
	loader := state.NewLoader(home.Path, alive)
	_, err = loader.Load(at)
	require.NoError(t, err)

	t.Run("when a live session's long transcript has been loaded", func(t *testing.T) {
		t.Run("and the session writes one more line", func(t *testing.T) {
			line := helpers.UserPrompt(sid, cwd, at.Format(time.RFC3339Nano))
			home.AppendTranscript(sid, cwd, line)
			before := measure.Take()
			_, err := loader.Load(at.Add(time.Second))
			require.NoError(t, err)
			read := measure.Take().Since(before)[measure.BytesRead]
			t.Logf("a %d MB transcript, one %d-byte line appended: the load read %d bytes", transcript>>20, len(line)+1, read)

			t.Run("it should read about what was written, not the transcript", func(t *testing.T) {
				assert.LessOrEqual(t, read, int64(len(line)+1+8<<10))
			})
		})
	})
}
