package control

import (
	"path/filepath"
)

var terminalPrefix = map[string][]string{
	"foot":           {},
	"kitty":          {},
	"alacritty":      {"-e"},
	"wezterm":        {"start", "--"},
	"gnome-terminal": {"--"},
	"konsole":        {"-e"},
	"xterm":          {"-e"},
	// terminator's -e takes one string; -x takes the rest of argv.
	"terminator": {"-x"},
}

// scriptedTerminals are launched through something else, so their
// argv[0] is not their own name.
var scriptedTerminals = map[string]func(command []string) []string{
	"Terminal":     appleTerminal,
	"Terminal.app": appleTerminal,
}

func knownConvention(terminal string) bool {
	_, scripted := scriptedTerminals[terminal]
	_, prefixed := terminalPrefix[filepath.Base(terminal)]
	return scripted || prefixed
}

func withConvention(terminal string, command []string) []string {
	if script, ok := scriptedTerminals[terminal]; ok {
		return script(command)
	}
	prefix, known := terminalPrefix[filepath.Base(terminal)]
	if !known {
		prefix = []string{"-e"}
	}
	argv := append([]string{terminal}, prefix...)
	return append(argv, command...)
}
