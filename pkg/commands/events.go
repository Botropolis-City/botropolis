package commands

import (
	"fmt"
	"io"
	"time"

	"botropolis.city/botropolis/pkg/proto"
)

// Events prints the daemon's event log: what happened to sessions
// after a moment, newest first.
type Events struct {
	socket string
}

func NewEvents(socket string) *Events {
	return &Events{socket: socket}
}

func (e *Events) Run(out io.Writer, since time.Time) error {
	client, err := proto.Dial(e.socket)
	if err != nil {
		return fmt.Errorf("the event log lives in the daemon, which is not listening on %s: %w", e.socket, err)
	}
	defer func() { _ = client.Close() }()
	logged, err := client.Events(since)
	if err != nil {
		return err
	}
	if len(logged) == 0 {
		_, err := fmt.Fprintln(out, "no events")
		return err
	}
	for _, event := range logged {
		line := fmt.Sprintf("%s  %-10s  %s", event.At.Local().Format("15:04"), event.Kind, event.Title)
		if event.Detail != "" {
			line += "  " + event.Detail
		}
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}
