// Package interrupt turns signals into a context: the first asks the
// program to stop and clean up after itself, and a second, while it is
// still stopping, exits at once and says so (item 114).
package interrupt

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
)

// Context is cancelled by the first signal on signals. A second calls
// exit with the shell's status for that signal, 128 plus its number.
func Context(parent context.Context, signals <-chan os.Signal, exit func(int), stderr io.Writer) context.Context {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-signals:
			cancel()
		case <-ctx.Done():
			return
		}
		sig := <-signals
		fmt.Fprintln(stderr, "botropolis: second interrupt: exiting without cleanup")
		code := 1
		if s, ok := sig.(syscall.Signal); ok {
			code = 128 + int(s)
		}
		exit(code)
	}()
	return ctx
}
