// Command man writes botropolis's manual pages: botropolis(1) and one
// page per command, botropolisd(1), botropolis-hook(1) and
// botropolis-config(5), gzipped, into -out. make man runs it. Item 167.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/app"
	"botropolis.city/botropolis/pkg/appd"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/hooks"
	"botropolis.city/botropolis/pkg/manpage"
)

func main() {
	out := flag.String("out", "bin/man", "the directory to write the pages into")
	day := flag.String("date", "", "the date on every page, as YYYY-MM-DD, so the pages build the same each time (required)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "man — write botropolis's manual pages\n\nUsage: go run ./tools/man -date YYYY-MM-DD [-out DIR]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if err := run(*out, *day); err != nil {
		fmt.Fprintln(os.Stderr, "man: "+err.Error())
		os.Exit(1)
	}
}

// neutralHome makes the defaults the flags print, which name the config
// file and the socket, read as they would for anyone rather than as the
// paths of the machine building the package.
func neutralHome() error {
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		return err
	}
	if err := os.Setenv("HOME", "~"); err != nil {
		return err
	}
	return os.Setenv("XDG_RUNTIME_DIR", "$XDG_RUNTIME_DIR")
}

func run(out, day string) error {
	date, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return fmt.Errorf("-date %q: want YYYY-MM-DD", day)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := neutralHome(); err != nil {
		return err
	}
	root, err := app.NewRoot()
	if err != nil {
		return err
	}
	if err := manpage.Write(out, date, root); err != nil {
		return err
	}
	if err := manpage.Write(out, date, appd.NewDaemonCLI(config.NewViper())); err != nil {
		return err
	}
	if err := manpage.Write(out, date, hooks.NewForwarderCLI(config.NewViper(), strings.NewReader(""))); err != nil {
		return err
	}
	return manpage.WriteConfig(out, date, config.Docs())
}
