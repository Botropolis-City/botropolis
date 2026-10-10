package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/appd"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout))
}

func run(ctx context.Context, args []string, out io.Writer) int {
	v := config.NewViper()
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
			return appd.RunDaemon(cmd.Context(), cfg, cmd.OutOrStdout())
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	config.BindFlags(v, root.Flags())
	root.SetOut(out)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(out, "botropolisd: %v\n", err)
		return 1
	}
	return 0
}
