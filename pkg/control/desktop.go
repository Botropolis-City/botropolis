package control

import (
	"regexp"
	"runtime"
	"strings"
)

// Desktop is what reveal, copy and attach reach for on one operating
// system.
type Desktop struct {
	OS         string
	Opener     string
	Clipboards [][]string
	// ClipboardHint is what to install when no clipboard tool is found.
	ClipboardHint string
	Terminals     []string
}

var linuxDesktop = Desktop{
	OS:     "linux",
	Opener: "xdg-open",
	Clipboards: [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	},
	ClipboardHint: "install wl-clipboard, xclip or xsel",
	Terminals:     []string{"foot", "kitty", "alacritty", "wezterm", "gnome-terminal", "konsole", "terminator", "xterm"},
}

// Terminal comes last on macOS because it is always there: the others
// are only tried when their command is on PATH.
var darwinDesktop = Desktop{
	OS:            "darwin",
	Opener:        "open",
	Clipboards:    [][]string{{"pbcopy"}},
	ClipboardHint: "pbcopy is not on PATH",
	Terminals:     []string{"wezterm", "kitty", "alacritty", "Terminal"},
}

// DesktopFor returns the desktop for a GOOS; anything that is not macOS
// is treated as a Linux desktop.
func DesktopFor(goos string) Desktop {
	if goos == "darwin" {
		return darwinDesktop
	}
	return linuxDesktop
}

// Native is the desktop this binary was built for.
func Native() Desktop {
	return DesktopFor(runtime.GOOS)
}

// KnownTerminals is the list tried when nothing names a terminal.
func (d Desktop) KnownTerminals() []string {
	return append([]string(nil), d.Terminals...)
}

func (d Desktop) TerminalCommand(getenv func(string) string, onPath func(string) bool, command []string) ([]string, error) {
	custom := strings.Fields(getenv("BOTROPOLIS_TERMINAL"))
	if len(custom) == 1 && knownConvention(custom[0]) {
		return withConvention(custom[0], command), nil
	}
	if len(custom) > 0 {
		return append(custom, command...), nil
	}
	if terminal := getenv("TERMINAL"); terminal != "" {
		return withConvention(terminal, command), nil
	}
	for _, terminal := range d.Terminals {
		if argv := withConvention(terminal, command); onPath(argv[0]) {
			return argv, nil
		}
	}
	return nil, errNoTerminal
}

// appleTerminal opens Terminal.app, which takes no command line, by
// asking it over AppleScript to run one.
func appleTerminal(command []string) []string {
	return []string{"osascript",
		"-e", `tell application "Terminal"`,
		"-e", "activate",
		"-e", `do script "` + AppleScriptString(shellJoin(command)) + `"`,
		"-e", "end tell"}
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`)

func shellJoin(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		if shellSafe.MatchString(arg) {
			quoted[i] = arg
		} else {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		}
	}
	return strings.Join(quoted, " ")
}

// AppleScriptString escapes s for the inside of an AppleScript string literal.
func AppleScriptString(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
