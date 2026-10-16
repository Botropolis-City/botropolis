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
	// Open opens the finder in the running city, or starts the city with
	// it open.
	Open() error
}

type FindServices interface {
	Find(cfg *config.Config) FindRunner
}

const findHelp = `Find sessions by what was said in them. Every word must appear in one passage.
Your prompts rank first, then recaps, replies, commands and paths, and titles;
live and waiting sessions come before stopped ones, and recent before old.
It asks the daemon when it is running, and reads Claude Code's files itself when not.

With no words it opens the finder in the running city instead, or starts the
city with it open, so it can be bound to a key in the desktop's own settings:

  GNOME: Settings → Keyboard → Keyboard Shortcuts → Custom Shortcuts,
         with the command botropolis find
  KDE: System Settings → Keyboard → Shortcuts → Add New → Command or Script,
       with the command botropolis find
  macOS: in Shortcuts, a shortcut with a Run Shell Script action running
         botropolis find, given a keyboard shortcut in its details
  i3, in its config:
    bindsym $mod+k [instance="botropolis"] focus; exec --no-startup-id botropolis find`

func NewFindCLI(load Loader, services FindServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find [words...]",
		Short: "Find sessions by what was said in them; with no words, open the finder in the city",
		Long:  findHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return services.Find(cfg).Open()
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
