package cli_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/cli"
	"botropolis.city/botropolis/pkg/shellinit"
)

func TestShellInitCLI(t *testing.T) {
	run := func(args ...string) (string, error) {
		root := &cobra.Command{Use: "botropolis"}
		root.AddCommand(cli.NewShellInitCLI())
		out := &bytes.Buffer{}
		root.SetOut(out)
		root.SetErr(&bytes.Buffer{})
		root.SetArgs(append([]string{"shell-init"}, args...))
		err := root.Execute()
		return out.String(), err
	}

	t.Run("when a shell is named", func(t *testing.T) {
		out, _ := run("zsh")
		want, _ := shellinit.Script("zsh")

		t.Run("it should print that shell's script", func(t *testing.T) {
			assert.Equal(t, want, out)
		})
	})

	t.Run("when no shell is named", func(t *testing.T) {
		_, err := run()

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
