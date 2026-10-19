package proc_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proc"
)

func TestStartTime(t *testing.T) {
	self, known := proc.StartTime(os.Getpid())

	t.Run("when asked about this process", func(t *testing.T) {
		t.Run("it should know when it started", func(t *testing.T) {
			assert.True(t, known)
		})
	})

	t.Run("when asked about this process twice", func(t *testing.T) {
		again, _ := proc.StartTime(os.Getpid())

		t.Run("it should give the same time both times", func(t *testing.T) {
			assert.Equal(t, self, again)
		})
	})

	t.Run("when asked about a process that has exited", func(t *testing.T) {
		cmd := exec.Command("true")
		require.NoError(t, cmd.Run())
		_, ok := proc.StartTime(cmd.Process.Pid)

		t.Run("it should not know", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when asked about a child started after this process", func(t *testing.T) {
		cmd := exec.Command("sleep", "30")
		require.NoError(t, cmd.Start())
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
		child, _ := proc.StartTime(cmd.Process.Pid)

		t.Run("it should not give an earlier time than this process's", func(t *testing.T) {
			assert.GreaterOrEqual(t, child, self)
		})
	})
}
