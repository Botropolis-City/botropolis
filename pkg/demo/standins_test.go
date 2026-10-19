package demo

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnstage(t *testing.T) {
	const pid, started = 4242, 918273
	unstaged := func(t *testing.T, listed string, startTime func(int) (uint64, bool)) []int {
		t.Helper()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, standIns), []byte(listed+"\n"), 0o600))
		var signalled []int
		require.NoError(t, unstage(home, startTime, func(p int, sig syscall.Signal) error {
			signalled = append(signalled, p)
			return nil
		}))
		return signalled
	}
	process := func(start uint64) func(int) (uint64, bool) {
		return func(p int) (uint64, bool) { return start, p == pid }
	}
	gone := func(int) (uint64, bool) { return 0, false }
	withStart := fmt.Sprintf("%d %d", pid, started)

	t.Run("when a stand-in is listed with its start time", func(t *testing.T) {
		t.Run("and its process started then, but has not yet become sleep (item 124)", func(t *testing.T) {
			got := unstaged(t, withStart, process(started))

			t.Run("it should be signalled", func(t *testing.T) {
				assert.Equal(t, []int{pid}, got)
			})
		})

		t.Run("and its process started then and is a sleep", func(t *testing.T) {
			got := unstaged(t, withStart, process(started))

			t.Run("it should be signalled", func(t *testing.T) {
				assert.Equal(t, []int{pid}, got)
			})
		})

		t.Run("and its pid now belongs to a process started at another time", func(t *testing.T) {
			got := unstaged(t, withStart, process(started+1))

			t.Run("it should not be signalled", func(t *testing.T) {
				assert.Empty(t, got)
			})
		})

		t.Run("and its pid is gone", func(t *testing.T) {
			got := unstaged(t, withStart, gone)

			t.Run("it should not be signalled", func(t *testing.T) {
				assert.Empty(t, got)
			})
		})
	})

	t.Run("when a stand-in is listed by pid alone", func(t *testing.T) {
		got := unstaged(t, fmt.Sprint(pid), process(started))

		t.Run("it should not be signalled", func(t *testing.T) {
			assert.Empty(t, got)
		})
	})
}
