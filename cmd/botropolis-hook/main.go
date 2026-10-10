package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/version"
)

const maxPayload = 4 << 20

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	v := config.NewViper()
	root := &cobra.Command{
		Use:           "botropolis-hook",
		Short:         "Forward a Claude Code hook event from stdin to botropolisd",
		Version:       version.Version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.New(v)
			if err != nil {
				return err
			}
			payload, err := io.ReadAll(io.LimitReader(in, maxPayload))
			if err != nil || len(payload) == 0 {
				return fmt.Errorf("no event on stdin")
			}
			return proto.SendEvent(cfg.Socket, payload)
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	config.BindFlags(v, root.Flags())
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(errOut, "botropolis-hook: %v\n", err)
	}
	return 0
}
