package demo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnstage(t *testing.T) {
	const pid, started = 4242, 918273
	unstaged := func(t *testing.T, listed string, proc fstest.MapFS) []int {
		t.Helper()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, standIns), []byte(listed+"\n"), 0o600))
		var signalled []int
		require.NoError(t, unstage(home, proc, func(p int, sig syscall.Signal) error {
			signalled = append(signalled, p)
			return nil
		}))
		return signalled
	}
	process := func(comm string, start int) fstest.MapFS {
		fields := make([]string, 20)
		for i := range fields {
			fields[i] = "0"
		}
		fields[0], fields[19] = "S", fmt.Sprint(start)
		return fstest.MapFS{
			"4242/comm": {Data: []byte(comm + "\n")},
			"4242/stat": {Data: []byte(fmt.Sprintf("%d (%s) %s\n", pid, comm, strings.Join(fields, " ")))},
		}
	}
	withStart := fmt.Sprintf("%d %d", pid, started)

	t.Run("when a stand-in is listed with its start time", func(t *testing.T) {
		t.Run("and its process started then, but has not yet become sleep (item 124)", func(t *testing.T) {
			got := unstaged(t, withStart, process("demo.test", started))

			t.Run("it should be signalled", func(t *testing.T) {
				assert.Equal(t, []int{pid}, got)
			})
		})

		t.Run("and its process started then and is a sleep", func(t *testing.T) {
			got := unstaged(t, withStart, process("sleep", started))

			t.Run("it should be signalled", func(t *testing.T) {
				assert.Equal(t, []int{pid}, got)
			})
		})

		t.Run("and its pid now belongs to a process started at another time", func(t *testing.T) {
			got := unstaged(t, withStart, process("sleep", started+1))

			t.Run("it should not be signalled", func(t *testing.T) {
				assert.Empty(t, got)
			})
		})

		t.Run("and its pid is gone", func(t *testing.T) {
			got := unstaged(t, withStart, fstest.MapFS{})

			t.Run("it should not be signalled", func(t *testing.T) {
				assert.Empty(t, got)
			})
		})
	})

	t.Run("when a stand-in is listed by pid alone", func(t *testing.T) {
		got := unstaged(t, fmt.Sprint(pid), process("sleep", started))

		t.Run("it should not be signalled", func(t *testing.T) {
			assert.Empty(t, got)
		})
	})
}
