package cli

import (
	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/version"
)

func NewVersionCLI() *cobra.Command {
	return deprecate(&cobra.Command{
		Use:   "version",
		Short: "Print the version this binary was built from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			version.Print(cmd.OutOrStdout(), cmd.Root().Name())
			return nil
		},
	}, "--version")
}
