package cli

import (
	"io"

	"github.com/spf13/cobra"
)

type HooksRunner interface {
	Install(out io.Writer) error
	Remove(out io.Writer) error
	DryRun(out io.Writer)
}

// runHooks installs, removes or prints the hooks as the command's flags
// say.
func runHooks(cmd *cobra.Command, hooks HooksRunner) error {
	remove, _ := cmd.Flags().GetBool("remove")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	switch {
	case dryRun:
		hooks.DryRun(cmd.OutOrStdout())
		return nil
	case remove:
		return hooks.Remove(cmd.OutOrStdout())
	}
	return hooks.Install(cmd.OutOrStdout())
}

func NewInstallHooksCLI(load Loader, services Services) *cobra.Command {
	cmd := deprecate(&cobra.Command{
		Use:   "install-hooks",
		Short: "Register botropolis-hook in ~/.claude/settings.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return runHooks(cmd, services.Hooks(cfg))
		},
	}, "setup")
	cmd.Flags().Bool("remove", false, "remove botropolis hooks instead of installing them")
	cmd.Flags().Bool("dry-run", false, "print the hook block and change nothing")
	return cmd
}
