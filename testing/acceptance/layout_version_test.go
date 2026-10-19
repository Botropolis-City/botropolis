package acceptance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 149: the map started from an empty layout when its file did not
// load, and saved that over the file, so a layout from a newer
// Botropolis was reset in silence.
func TestNewerLayout(t *testing.T) {
	t.Run("when the person's layout is from a version this build cannot read", func(t *testing.T) {
		r := newStateRun(t)
		withSession(t, r.home)
		path := filepath.Join(r.realState, "botropolis", "layout.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		newer := []byte(`{"version":99,"readableFrom":99,"districts":{"/w/lib":{"slots":{"s":4}}}}` + "\n")
		require.NoError(t, os.WriteFile(path, newer, 0o600))

		cmd := r.command(t, false)
		var out strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &out
		require.NoError(t, cmd.Start())
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		var err error
		select {
		case err = <-exited:
		case <-time.After(20 * time.Second):
			_ = cmd.Process.Kill()
			err = <-exited
			t.Fatalf("the city was still running 20 s after starting on a layout it cannot read (%v)\n%s", err, out.String())
		}
		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)

		t.Run("it should refuse to start", func(t *testing.T) {
			assert.Error(t, err)
		})

		t.Run("it should name the file", func(t *testing.T) {
			assert.Contains(t, out.String(), path+" is layout version 99")
		})

		t.Run("it should leave the file as it was", func(t *testing.T) {
			assert.Equal(t, string(newer), string(after))
		})
	})
}
