package control_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInTerminal(t *testing.T) {
	t.Run("when a stopped session is resumed from the finder", func(t *testing.T) {
		runner := &fakeRunner{}
		require.NoError(t, newControl(runner).ResumeIn(dir, sessionID))
		require.Len(t, runner.starts, 1)

		t.Run("it should open a terminal in its directory running claude --resume <id>", func(t *testing.T) {
			assert.Equal(t, call{dir, []string{"kitty", "sh", "-c", `cd -- "$1" && exec claude --resume "$2"`, "sh", dir, sessionID}}, runner.starts[0])
		})
	})

	t.Run("when a new session is started from the finder", func(t *testing.T) {
		runner := &fakeRunner{}
		require.NoError(t, newControl(runner).StartIn(dir))
		require.Len(t, runner.starts, 1)

		t.Run("it should open a terminal in the directory running claude", func(t *testing.T) {
			assert.Equal(t, call{dir, []string{"kitty", "sh", "-c", `cd -- "$1" && exec claude`, "sh", dir}}, runner.starts[0])
		})
	})
}
