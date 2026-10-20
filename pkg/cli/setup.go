package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/shellinit"
)

// DaemonRunner makes sure the daemon is running. Item 167.
type DaemonRunner interface {
	Ensure(out io.Writer, dryRun bool) error
}

type SetupServices interface {
	Hooks(cfg *config.Config) HooksRunner
	Daemon(cfg *config.Config) DaemonRunner
}

func NewSetupCLI(load Loader, services SetupServices, getenv func(string) string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install the hooks, start the daemon, and give the shell line",
		Long: "Install the hooks in ~/.claude/settings.json, start the daemon if it is not\n" +
			"running, saying exactly what it ran, and print the line for your shell's rc\nfile. Safe to run again.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if err := runHooks(cmd, services.Hooks(cfg)); err != nil {
				return err
			}
			if remove, _ := cmd.Flags().GetBool("remove"); remove {
				return nil
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if err := services.Daemon(cfg).Ensure(cmd.OutOrStdout(), dryRun); err != nil {
				return err
			}
			for _, l := range shellinit.RCLinesFor(getenv("SHELL")) {
				fmt.Fprintf(cmd.OutOrStdout(), "add to %s: %s\n", l.File, l.Line)
			}
			return nil
		},
	}
	cmd.Flags().Bool("dry-run", false, "print what would change and change nothing")
	cmd.Flags().Bool("remove", false, "take the hooks out again")
	return cmd
}
