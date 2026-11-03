package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
)

const sunPathMax = 104

func stageDefaultSocket(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", shortDir(t))
	t.Setenv("HOME", shortDir(t))
	return proto.SocketPath()
}

func requireBindable(t *testing.T, path string) {
	t.Helper()
	require.Lessf(t, len(path), sunPathMax, "socket path is %d bytes, over sun_path's %d on macOS: %s", len(path), sunPathMax, path)
}

// Item 337: macOS keeps the default socket under HOME, and its TMPDIR is
// long enough that a socket staged beneath it cannot be bound.
func TestStageDefaultSocket(t *testing.T) {
	t.Run("when TMPDIR is as long as a macOS runner's", func(t *testing.T) {
		base, err := os.MkdirTemp("/tmp", "b337")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(base) })
		long := filepath.Join(base, strings.Repeat("n", 49-len(base)-1))
		require.NoError(t, os.MkdirAll(long, 0o700))
		t.Setenv("TMPDIR", long)
		t.Setenv("HOME", filepath.Join(long, "botropolis-home-1605930590"))
		stageDefaultSocket(t)
		darwin := proto.SocketPathFor("darwin", os.Getenv, os.Getenv("HOME"))

		t.Run("it should stage a macOS default socket short enough to bind", func(t *testing.T) {
			assert.Less(t, len(darwin), sunPathMax)
		})
	})
}
