package appd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/version"
)

// NewDaemonCLI is botropolisd's command, as its main runs it and its
// manual page describes it.
func NewDaemonCLI(v *viper.Viper) *cobra.Command {
	root := &cobra.Command{
		Use:           "botropolisd",
		Short:         "Watch ~/.claude and serve the city model over a unix socket",
		Version:       version.Version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.New(v)
			if err != nil {
				return err
			}
			return RunDaemon(cmd.Context(), cfg, cmd.OutOrStdout())
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	config.BindFlags(v, root.Flags())
	return root
}
