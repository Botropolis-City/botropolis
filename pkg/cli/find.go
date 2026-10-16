package cli

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/find"
)

type FindRunner interface {
	Run(out io.Writer, q find.Query) error
}

type FindServices interface {
	Find(cfg *config.Config) FindRunner
}

func NewFindCLI(load Loader, services FindServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find words...",
		Short: "Find sessions by what was said in them: your prompts, Claude's replies and recaps, commands and paths",
		Long: "Find sessions by what was said in them. Every word must appear in one passage.\n" +
			"Your prompts rank first, then recaps, replies, commands and paths, and titles;\n" +
			"live and waiting sessions come before stopped ones, and recent before old.\n" +
			"It asks the daemon when it is running, and reads Claude Code's files itself when not.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetInt("limit")
			toolOutput, _ := cmd.Flags().GetBool("tool-output")
			return services.Find(cfg).Run(cmd.OutOrStdout(), find.Query{Words: strings.Join(args, " "), Limit: limit, ToolOutput: toolOutput})
		},
	}
	cmd.Flags().IntP("limit", "n", 10, "how many sessions to print at most (0 for all)")
	cmd.Flags().Bool("tool-output", false, "also search tool output, read from the transcripts")
	return cmd
}
