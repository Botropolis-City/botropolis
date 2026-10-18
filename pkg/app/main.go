package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"botropolis.city/botropolis/pkg/interrupt"
)

func Main() int {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	ctx := interrupt.Context(context.Background(), signals, os.Exit, os.Stderr)
	return Run(ctx, os.Args[1:])
}
