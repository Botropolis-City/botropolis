package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// deprecate hides a command that keeps working through 0.x, and has it
// name its replacement on stderr each time it runs. Item 167.
func deprecate(cmd *cobra.Command, now string) *cobra.Command {
	cmd.Hidden = true
	run := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if c == cmd {
			deprecationNote(c.ErrOrStderr(), c.Name(), now)
		}
		return run(c, args)
	}
	return cmd
}

func deprecationNote(w io.Writer, old, now string) {
	fmt.Fprintf(w, "botropolis: %s is deprecated and goes at 1.0: use botropolis %s\n", old, now)
}
