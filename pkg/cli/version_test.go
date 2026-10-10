package cli_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/cli"
	"botropolis.city/botropolis/pkg/version"
)

func TestVersionCLI(t *testing.T) {
	t.Run("when the version subcommand is run", func(t *testing.T) {
		root := &cobra.Command{Use: "botropolis"}
		cmd := cli.NewVersionCLI()
		root.AddCommand(cmd)

		out := &bytes.Buffer{}
		root.SetOut(out)
		root.SetArgs([]string{"version"})
		err := root.Execute()

		t.Run("and it writes to the command's output", func(t *testing.T) {
			t.Run("it should not error", func(t *testing.T) {
				assert.NoError(t, err)
			})

			t.Run("it should name the root binary and its version", func(t *testing.T) {
				assert.Equal(t, "botropolis "+version.Version+"\n", out.String())
			})
		})
	})
}
