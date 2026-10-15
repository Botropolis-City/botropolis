package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/shellinit"
)

func NewShellInitCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init SHELL",
		Short: "Print the shell integration that makes a plain claude a background session",
		Long: "Print the shell integration for SHELL (" + strings.Join(shellinit.Shells, ", ") + ").\n\n" +
			"  bash  ~/.bashrc                    eval \"$(botropolis shell-init bash)\"\n" +
			"  zsh   ~/.zshrc                     eval \"$(botropolis shell-init zsh)\"\n" +
			"  fish  ~/.config/fish/config.fish   botropolis shell-init fish | source",
		Args:      cobra.ExactArgs(1),
		ValidArgs: shellinit.Shells,
		RunE: func(cmd *cobra.Command, args []string) error {
			script, err := shellinit.Script(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), script)
			return err
		},
	}
}
