package cli_test

import (
	"bytes"
	"strings"
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

	for _, c := range []struct{ shell, completion string }{
		{"bash", "__start_botropolis"},
		{"zsh", "compdef _botropolis botropolis"},
		{"fish", "complete -c botropolis"},
	} {
		t.Run("when "+c.shell+" is named", func(t *testing.T) {
			out, _ := run(c.shell)
			want, _ := shellinit.Script(c.shell)

			t.Run("it should print that shell's script first", func(t *testing.T) {
				assert.True(t, strings.HasPrefix(out, want))
			})

			t.Run("it should load completions after it", func(t *testing.T) {
				assert.Contains(t, strings.TrimPrefix(out, want), c.completion)
			})
		})
	}

	t.Run("when no shell is named", func(t *testing.T) {
		_, err := run()

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
