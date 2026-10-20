package cli

import (
	"errors"
	"io"

	"botropolis.city/botropolis/pkg/state"

	"github.com/spf13/cobra"
)

type StatusRunner interface {
	Run(out io.Writer, direct, all, asJSON bool) error
	Snapshot(direct bool) (state.Snapshot, error)
}

func NewStatusCLI(load Loader, services Services, live TUIServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List the sessions; --live keeps the table open",
		Long: "List the sessions: one row per live one, or with --all the resting ones too.\n" +
			"With --live the table stays open and updates; j and k move, enter attaches,\n" +
			"s stops, x deletes, q quits.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			direct, _ := cmd.Flags().GetBool("direct")
			all, _ := cmd.Flags().GetBool("all")
			asJSON, _ := cmd.Flags().GetBool("json")
			isLive, _ := cmd.Flags().GetBool("live")
			if isLive {
				if asJSON {
					return errors.New("--live and --json do not combine")
				}
				return live.TUI(cfg).Run(cmd.Context())
			}
			if cfg.HomeSet && !cfg.SocketSet {
				direct = true
			}
			return services.Status(cfg).Run(cmd.OutOrStdout(), direct, all, asJSON)
		},
	}
	cmd.Flags().BoolP("all", "a", false, "include resting sessions")
	cmd.Flags().BoolP("live", "l", false, "keep the table open, live")
	cmd.Flags().Bool("json", false, "a JSON array, one object per session")
	cmd.Flags().Bool("direct", false, "read ~/.claude without asking the daemon (implied by --home without --socket)")
	return cmd
}
