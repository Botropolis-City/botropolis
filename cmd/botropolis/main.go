package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"botropolis.city/botropolis/pkg/app"
	"botropolis.city/botropolis/pkg/interrupt"
)

func main() {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	ctx := interrupt.Context(context.Background(), signals, os.Exit, os.Stderr)
	os.Exit(app.Run(ctx, os.Args[1:]))
}
