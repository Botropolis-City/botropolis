package cli

import (
	"context"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/config"
)

type TUIRunner interface {
	Run(ctx context.Context) error
}

type TUIServices interface {
	TUI(cfg *config.Config) TUIRunner
}

func NewTUICLI(load Loader, services TUIServices) *cobra.Command {
	return deprecate(&cobra.Command{
		Use:   "tui",
		Short: "The session table in your terminal, live",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return services.TUI(cfg).Run(cmd.Context())
		},
	}, "status --live")
}
