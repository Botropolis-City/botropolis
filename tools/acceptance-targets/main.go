package main

import (
	"os"

	"botropolis.city/botropolis/tools/acceptance-targets/targets"
)

func main() { os.Exit(targets.Main(os.Args[1:], os.Stdout, os.Stderr)) }
