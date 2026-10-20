package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"botropolis.city/botropolis/pkg/control"
	"botropolis.city/botropolis/pkg/proto"
)

// Check is one thing the doctor looked at.
type Check struct {
	Name   string
	Status string // ok, warn or fail
	Detail string
	Hint   string
}

const (
	CheckOK   = "ok"
	CheckWarn = "warn"
	CheckFail = "fail"
)

// Doctor looks over a first run: the daemon, the hooks, a terminal, the
// claude CLI, the harness homes and the display.
type Doctor struct {
	Home        string
	Socket      string
	Terminal    string
	HookCommand string
	CodexHome   string
	Desktop     control.Desktop
	Getenv      func(string) string
	OnPath      func(string) bool
	// Dial reaches the daemon and asks what it is.
	Dial func(sock string) error
	// RestartHint says what to do about a daemon out of sync (item 152).
	RestartHint func(*proto.Mismatch) string
}

// Run returns every check, in the order a first run needs them.
func (d Doctor) Run() []Check {
	var checks []Check
	var mismatch *proto.Mismatch
	if err := d.Dial(d.Socket); errors.As(err, &mismatch) {
		hint := ""
		if d.RestartHint != nil {
			hint = d.RestartHint(mismatch)
		}
		checks = append(checks, Check{"daemon", CheckFail, mismatch.Error(), hint})
	} else if err != nil {
		checks = append(checks, Check{"daemon", CheckWarn, "not reachable at " + d.Socket, d.daemonStart() + "; without it every client scans ~/.claude itself"})
	} else {
		checks = append(checks, Check{"daemon", CheckOK, "answering at " + d.Socket, ""})
	}

	settings := filepath.Join(d.Home, ".claude", "settings.json")
	if data, err := os.ReadFile(settings); err != nil {
		checks = append(checks, Check{"hooks", CheckWarn, "no " + settings, "botropolis setup (needs-you is derived without them, but arrives in under a second with)"})
	} else if strings.Contains(string(data), d.HookCommand) {
		checks = append(checks, Check{"hooks", CheckOK, d.HookCommand + " is in " + settings, ""})
	} else {
		checks = append(checks, Check{"hooks", CheckWarn, d.HookCommand + " is not in " + settings, "botropolis setup"})
	}

	getenv := func(key string) string {
		if key == "BOTROPOLIS_TERMINAL" && d.Terminal != "" {
			return d.Terminal
		}
		return d.Getenv(key)
	}
	if argv, err := d.Desktop.TerminalCommand(getenv, d.OnPath, []string{"claude", "attach", "<id>"}); err != nil {
		checks = append(checks, Check{"terminal", CheckFail, "none found", "set terminal in the config or BOTROPOLIS_TERMINAL / TERMINAL; attach opens the session in it"})
	} else {
		checks = append(checks, Check{"terminal", CheckOK, strings.Join(argv, " "), ""})
	}

	if d.OnPath("claude") {
		checks = append(checks, Check{"claude", CheckOK, "on PATH", ""})
	} else {
		checks = append(checks, Check{"claude", CheckFail, "not on PATH", "install Claude Code; every start, attach, stop and resume goes through it"})
	}

	claudeHome := filepath.Join(d.Home, ".claude")
	if info, err := os.Stat(claudeHome); err == nil && info.IsDir() {
		checks = append(checks, Check{"harness claude", CheckOK, claudeHome, ""})
	} else {
		checks = append(checks, Check{"harness claude", CheckFail, "no " + claudeHome, "run claude once; there is nothing to draw before then"})
	}
	if info, err := os.Stat(d.CodexHome); err == nil && info.IsDir() {
		checks = append(checks, Check{"harness codex", CheckOK, d.CodexHome, ""})
	} else {
		checks = append(checks, Check{"harness codex", CheckOK, "none (" + d.CodexHome + " is not there)", ""})
	}

	switch {
	case d.Desktop.OS == "darwin":
		checks = append(checks, Check{"display", CheckOK, "macOS", ""})
	case d.Getenv("WAYLAND_DISPLAY") != "" && d.Getenv("DISPLAY") != "":
		checks = append(checks, Check{"display", CheckOK, "Wayland with XWayland", "the city runs under XWayland (Ebitengine via GLFW/X11); float it by WM_CLASS botropolis as on X11"})
	case d.Getenv("WAYLAND_DISPLAY") != "":
		checks = append(checks, Check{"display", CheckWarn, "Wayland without XWayland", "the city needs XWayland; status, tui, bar and notify work without a display"})
	case d.Getenv("DISPLAY") != "":
		checks = append(checks, Check{"display", CheckOK, "X11 " + d.Getenv("DISPLAY"), ""})
	default:
		checks = append(checks, Check{"display", CheckWarn, "none", "the city needs one; status, tui, bar and notify work without"})
	}
	return checks
}

func (d Doctor) daemonStart() string {
	if d.Desktop.OS == "darwin" {
		return "brew services start botropolis"
	}
	return "systemctl --user enable --now botropolisd"
}

// Report prints the checks as a table with a hint for anything short of ok.
func (d Doctor) Report(out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CHECK\tSTATUS\tDETAIL")
	problems := 0
	for _, c := range d.Run() {
		fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, c.Status, c.Detail)
		if c.Status != CheckOK {
			problems++
		}
		if c.Hint != "" {
			fmt.Fprintf(w, "\t\t  %s\n", c.Hint)
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if problems == 0 {
		fmt.Fprintln(out, "all good")
	}
	return nil
}
