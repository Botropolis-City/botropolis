package commands

import (
	"fmt"
	"io"
	"strings"
)

// Daemon starts botropolisd through the platform's service manager when
// it is not running, for setup. Item 167.
type Daemon struct {
	GOOS    string
	OnPath  func(string) bool
	Running func() bool
	Run     func(argv []string) error
}

// command is how this platform starts the daemon, nil when no service
// manager botropolis knows is installed.
func (d Daemon) command() []string {
	switch {
	case d.GOOS == "darwin" && d.OnPath("brew"):
		return []string{"brew", "services", "start", "botropolis"}
	case d.GOOS != "darwin" && d.OnPath("systemctl"):
		return []string{"systemctl", "--user", "enable", "--now", "botropolisd"}
	}
	return nil
}

func (d Daemon) Ensure(out io.Writer, dryRun bool) error {
	if d.Running() {
		_, err := fmt.Fprintln(out, "the daemon is running")
		return err
	}
	argv := d.command()
	if argv == nil {
		_, err := fmt.Fprintln(out, "the daemon is not running: start botropolisd with your service manager, or run it in a terminal")
		return err
	}
	line := strings.Join(argv, " ")
	if dryRun {
		_, err := fmt.Fprintln(out, "would start the daemon: "+line)
		return err
	}
	if err := d.Run(argv); err != nil {
		return fmt.Errorf("%s: %w", line, err)
	}
	_, err := fmt.Fprintln(out, "started the daemon: "+line)
	return err
}
