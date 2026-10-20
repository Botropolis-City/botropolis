package shellinit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/shellinit"
)

func TestScript(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run("when the shell is "+shell, func(t *testing.T) {
			script, err := shellinit.Script(shell)
			require.NoError(t, err)

			t.Run("it should define claude", func(t *testing.T) {
				assert.Regexp(t, `(?m)^(function claude\b|claude\(\))`, script)
			})

			t.Run("it should attach with botropolis attach", func(t *testing.T) {
				assert.Contains(t, script, "botropolis attach")
			})

			t.Run("it should never attach with claude attach itself", func(t *testing.T) {
				assert.NotContains(t, script, "command claude attach")
			})
		})
	}

	t.Run("when the shell is not one botropolis knows", func(t *testing.T) {
		_, err := shellinit.Script("tcsh")

		t.Run("it should name the shells it does know", func(t *testing.T) {
			assert.ErrorContains(t, err, "bash, zsh, fish")
		})
	})

	t.Run("when the shell is given as a path, as $SHELL holds it", func(t *testing.T) {
		script, err := shellinit.Script("/usr/bin/fish")
		require.NoError(t, err)

		t.Run("it should use the shell that path names", func(t *testing.T) {
			assert.Contains(t, script, "function claude")
		})
	})
}
