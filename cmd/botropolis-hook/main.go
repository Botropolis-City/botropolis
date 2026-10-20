package main

import (
	"fmt"
	"io"
	"os"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/hooks"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	root := hooks.NewForwarderCLI(config.NewViper(), in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(errOut, "botropolis-hook: %v\n", err)
	}
	return 0
}
