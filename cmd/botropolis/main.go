package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"botropolis.city/botropolis/pkg/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(app.Run(ctx, os.Args[1:]))
}
