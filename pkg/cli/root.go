package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/version"
)

func NewRootCLI(v *viper.Viper, subcommands ...*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:           "botropolis",
		Short:         "Every Claude Code session on the machine, as a city",
		Long:          "Every Claude Code session on the machine, as a city.\n\nWith no arguments, botropolis opens the city.",
		Version:       version.Version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	config.BindFlags(v, root.PersistentFlags())
	root.AddCommand(subcommands...)
	root.Flags().Bool("tui", false, "open the terminal table instead of the city")
	AddScreenshotFlag(root.Flags())
	AddKeysFlag(root.Flags())
	AddHoverFlag(root.Flags())
	AddRecordFlags(root.Flags())
	AddHeadlessFlag(root.Flags())
	AddWindowFlag(root.Flags())
	AddScriptFlag(root.Flags())
	byName := map[string]*cobra.Command{}
	for _, sub := range subcommands {
		byName[sub.Name()] = sub
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		wantTUI, _ := cmd.Flags().GetBool("tui")
		if wantTUI {
			if tui, ok := byName["tui"]; ok {
				return tui.RunE(cmd, args)
			}
		}
		if city, ok := byName["city"]; ok {
			return city.RunE(cmd, args)
		}
		return cmd.Help()
	}
	return root
}
