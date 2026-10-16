package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/single"
)

func TestOneCity(t *testing.T) {
	t.Run("when a second botropolis city starts beside a running one", func(t *testing.T) {
		runtime, err := os.MkdirTemp("", "bc")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(runtime) })
		first, deferred, err := single.Claim(filepath.Join(runtime, "botropolis", "city.sock"), single.Request{Op: single.Raise}, false)
		require.NoError(t, err)
		require.False(t, deferred)
		t.Cleanup(func() { _ = first.Close() })

		second := exec.Command(botropolis, "city", "--home", t.TempDir())
		second.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+runtime, "DISPLAY=", "WAYLAND_DISPLAY=")
		out, err := second.CombinedOutput()

		t.Run("it should exit without opening a window", func(t *testing.T) {
			var heard single.Request
			select {
			case heard = <-first.Requests():
			case <-time.After(time.Second):
			}
			assert.Equal(t, [3]any{nil, "the city is already open; brought it forward\n", single.Request{Op: single.Raise}}, [3]any{err, string(out), heard})
		})
	})
}
