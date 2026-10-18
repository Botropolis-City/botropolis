package demo

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnstage(t *testing.T) {
	const pid = 4242
	unstaged := func(t *testing.T, proc fstest.MapFS) []int {
		t.Helper()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, standIns), []byte("4242\n"), 0o600))
		var signalled []int
		require.NoError(t, unstage(home, proc, func(p int, sig syscall.Signal) error {
			signalled = append(signalled, p)
			return nil
		}))
		return signalled
	}
	process := func(comm, cmdline string) fstest.MapFS {
		return fstest.MapFS{
			"4242/comm":    {Data: []byte(comm + "\n")},
			"4242/cmdline": {Data: []byte(cmdline)},
		}
	}

	t.Run("when a stand-in is listed", func(t *testing.T) {
		t.Run("and its process is mid-exec, named sleep with no command line yet (item 104)", func(t *testing.T) {
			got := unstaged(t, process("sleep", ""))

			t.Run("it should be signalled", func(t *testing.T) {
				assert.Equal(t, []int{pid}, got)
			})
		})

		t.Run("and its process is a sleep", func(t *testing.T) {
			got := unstaged(t, process("sleep", "sleep\x0060\x00"))

			t.Run("it should be signalled", func(t *testing.T) {
				assert.Equal(t, []int{pid}, got)
			})
		})

		t.Run("and its pid now belongs to another program", func(t *testing.T) {
			got := unstaged(t, process("bash", "/usr/bin/bash\x00"))

			t.Run("it should not be signalled", func(t *testing.T) {
				assert.Empty(t, got)
			})
		})
	})
}
