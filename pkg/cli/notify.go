package cli

import (
	"context"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/config"
)

type NotifyRunner interface {
	Run(ctx context.Context)
}

type NotifyServices interface {
	Notify(cfg *config.Config) NotifyRunner
}

func NewNotifyCLI(load Loader, services NotifyServices) *cobra.Command {
	return &cobra.Command{
		Use:   "notify",
		Short: "Send a desktop notification whenever a session starts needing you",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			services.Notify(cfg).Run(cmd.Context())
			return nil
		},
	}
}
