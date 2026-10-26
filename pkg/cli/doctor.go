package cli

import (
	"io"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/config"
)

type DoctorRunner interface {
	Report(out io.Writer) error
}

type DoctorServices interface {
	Doctor(cfg *config.Config) DoctorRunner
}

func NewDoctorCLI(load Loader, services DoctorServices) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the daemon, its statistics store, the hooks, a terminal, the claude CLI, the harnesses and the display",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return services.Doctor(cfg).Report(cmd.OutOrStdout())
		},
	}
}
