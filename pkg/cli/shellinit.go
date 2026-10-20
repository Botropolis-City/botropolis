package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/shellinit"
)

func NewShellInitCLI() *cobra.Command {
	var lines []string
	for _, l := range shellinit.RCLines {
		lines = append(lines, fmt.Sprintf("  %-5s %-28s %s", l.Shell, l.File, l.Line))
	}
	return &cobra.Command{
		Use:   "shell-init bash|zsh|fish",
		Short: "Print the shell integration for your rc file",
		Long: "Print the shell integration, which makes a plain claude a background session\n" +
			"and loads completions. Add one line to your rc file:\n\n" + strings.Join(lines, "\n"),
		Args:      cobra.ExactArgs(1),
		ValidArgs: shellinit.Shells,
		RunE: func(cmd *cobra.Command, args []string) error {
			script, err := shellinit.Script(args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if _, err := fmt.Fprint(out, script); err != nil {
				return err
			}
			return completions(cmd.Root(), filepath.Base(args[0]), out)
		},
	}
}

// completions writes the shell's completion script after the
// integration, so one rc line loads both. zsh's needs compinit, which not
// every ~/.zshrc loads.
func completions(root *cobra.Command, shell string, out io.Writer) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(out, true)
	case "zsh":
		fmt.Fprintln(out, "if (( $+functions[compdef] )); then")
		if err := root.GenZshCompletion(out); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, "fi")
		return err
	case "fish":
		return root.GenFishCompletion(out, true)
	}
	return nil
}
