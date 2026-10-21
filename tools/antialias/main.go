// Command antialias fails when any Go file in the module draws with an
// anti-aliased vector call outside the functions tools/antialias/allow.txt
// names (item 191). Run it as tools/check-antialias.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"botropolis.city/botropolis/tools/antialias/check"
)

const vectorPath = "github.com/hajimehoshi/ebiten/v2/vector"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check-antialias", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "the module tree to check")
	allow := flags.String("allow", "", "the allowlist (default: tools/antialias/allow.txt under --root)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: tools/check-antialias [--root DIR] [--allow FILE] [--help]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Fails on any anti-aliased vector call in a non-test Go file under --root,")
		fmt.Fprintln(stderr, "outside the functions the allowlist names, each with its reason. Draw")
		fmt.Fprintln(stderr, "through pkg/shapes instead (item 191).")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *allow == "" {
		*allow = filepath.Join(*root, "tools", "antialias", "allow.txt")
	}
	dir, err := exec.Command("go", "list", "-f", "{{.Dir}}", vectorPath).Output()
	if err != nil {
		fmt.Fprintf(stderr, "check-antialias: finding the vector package: %v\n", err)
		return 2
	}
	list, err := os.Open(*allow)
	if err != nil {
		fmt.Fprintf(stderr, "check-antialias: %v\n", err)
		return 2
	}
	defer func() { _ = list.Close() }()
	found, err := check.Run(*root, strings.TrimSpace(string(dir)), vectorPath, list)
	if err != nil {
		fmt.Fprintf(stderr, "check-antialias: %v\n", err)
		return 1
	}
	for _, f := range found {
		fmt.Fprintln(stderr, f)
	}
	if len(found) > 0 {
		fmt.Fprintf(stderr, "check-antialias: %d anti-aliased vector call(s); draw through pkg/shapes, or allowlist what is drawn once\n", len(found))
		return 1
	}
	fmt.Fprintln(stdout, "no anti-aliased vector calls outside the allowlist")
	return 0
}
