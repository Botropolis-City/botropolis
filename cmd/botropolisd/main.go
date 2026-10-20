package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"botropolis.city/botropolis/pkg/appd"
	"botropolis.city/botropolis/pkg/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout))
}

func run(ctx context.Context, args []string, out io.Writer) int {
	root := appd.NewDaemonCLI(config.NewViper())
	root.SetOut(out)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(out, "botropolisd: %v\n", err)
		return 1
	}
	return 0
}
