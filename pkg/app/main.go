package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"botropolis.city/botropolis/pkg/interrupt"
	"botropolis.city/botropolis/pkg/proto"
)

func Main() int {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	ctx := interrupt.Context(context.Background(), signals, os.Exit, os.Stderr)
	defer proto.OnNotice(func(notice string) { fmt.Fprintf(os.Stderr, "botropolis: %s\n", notice) })()
	return Run(ctx, os.Args[1:])
}
