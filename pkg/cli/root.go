package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/version"
)

// groups are the root help's headings, in order, and the commands under
// each. Item 167.
var groups = []struct {
	group    cobra.Group
	commands []string
}{
	{cobra.Group{ID: "sessions", Title: "Sessions:"}, []string{"new", "attach", "stop", "rm", "status", "find"}},
	{cobra.Group{ID: "city", Title: "The city:"}, []string{"city"}},
	{cobra.Group{ID: "setup", Title: "Setting up and checking:"}, []string{"setup", "doctor", "events"}},
	{cobra.Group{ID: "programs", Title: "For other programs:"}, []string{"bar", "notify", "shell-init"}},
}

func NewRootCLI(v *viper.Viper, subcommands ...*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:           "botropolis",
		Short:         "Every Claude Code session on the machine, as a city",
		Long:          "Every Claude Code session on the machine, as a city.\n\nWith no command, botropolis opens the city.",
		Version:       version.Version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			for _, note := range config.DeprecationNotes(cmd.Flags()) {
				fmt.Fprintln(cmd.ErrOrStderr(), "botropolis: "+note)
			}
		},
	}
	root.CompletionOptions.HiddenDefaultCmd = true
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.Flags().BoolP("version", "v", false, "print the version")
	config.BindFlags(v, root.PersistentFlags())
	byName := map[string]*cobra.Command{}
	for _, sub := range subcommands {
		byName[sub.Name()] = sub
	}
	for _, g := range groups {
		root.AddGroup(&g.group)
		for _, name := range g.commands {
			if sub, ok := byName[name]; ok {
				sub.GroupID = g.group.ID
			}
		}
	}
	root.AddCommand(subcommands...)
	root.Flags().Bool("tui", false, "open the terminal table instead of the city")
	addFrameFlags(root.Flags(), false)
	root.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "help" && f.Name != "version" {
			f.Hidden = true
		}
	})
	root.RunE = func(cmd *cobra.Command, args []string) error {
		wantTUI, _ := cmd.Flags().GetBool("tui")
		if wantTUI {
			if tui, ok := byName["tui"]; ok {
				deprecationNote(cmd.ErrOrStderr(), "--tui", "status --live")
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
