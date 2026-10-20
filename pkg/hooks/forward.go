package hooks

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/version"
)

const maxPayload = 4 << 20

// NewForwarderCLI is botropolis-hook's command, which forwards the event
// on in to the daemon, as its main runs it and its manual page describes
// it.
func NewForwarderCLI(v *viper.Viper, in io.Reader) *cobra.Command {
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
	return root
}
