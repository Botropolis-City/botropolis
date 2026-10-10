package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/config"
)

type BarRunner interface {
	Once(out io.Writer, format commands.BarFormat, direct bool) error
	Watch(ctx context.Context, out io.Writer, format commands.BarFormat)
}

type BarServices interface {
	Bar(cfg *config.Config) BarRunner
}

func NewBarCLI(load Loader, services BarServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bar",
		Short: "One line for a status bar: waybar JSON or plain text",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			format, _ := cmd.Flags().GetString("format")
			if format != string(commands.BarWaybar) && format != string(commands.BarText) {
				return fmt.Errorf("unknown format %q (waybar or text)", format)
			}
			watch, _ := cmd.Flags().GetBool("watch")
			bar := services.Bar(cfg)
			if watch {
				bar.Watch(cmd.Context(), cmd.OutOrStdout(), commands.BarFormat(format))
				return nil
			}
			return bar.Once(cmd.OutOrStdout(), commands.BarFormat(format), cfg.HomeSet && !cfg.SocketSet)
		},
	}
	cmd.Flags().String("format", string(commands.BarWaybar), "waybar (JSON) or text")
	cmd.Flags().Bool("watch", false, "keep printing a line whenever the daemon reports a change")
	return cmd
}
