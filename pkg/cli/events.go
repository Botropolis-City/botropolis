package cli

import (
	"io"
	"time"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/config"
)

type EventsRunner interface {
	Run(out io.Writer, since time.Time) error
}

type EventsServices interface {
	Events(cfg *config.Config) EventsRunner
}

func NewEventsCLI(load Loader, services EventsServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Print the daemon's event log: what needed you, errored, merged, started or ended",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			since, _ := cmd.Flags().GetDuration("since")
			return services.Events(cfg).Run(cmd.OutOrStdout(), time.Now().Add(-since))
		},
	}
	cmd.Flags().Duration("since", 24*time.Hour, "how far back to list")
	return cmd
}
