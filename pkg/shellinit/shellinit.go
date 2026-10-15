// Package shellinit prints the shell integration that makes a plain
// `claude` a background session, for each shell botropolis supports.
package shellinit

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed *.tmpl
var templates embed.FS

// Shells are the shells there is a script for, in the order they are named.
var Shells = []string{"bash", "zsh", "fish"}

var files = map[string]string{"bash": "posix.sh.tmpl", "zsh": "posix.sh.tmpl", "fish": "fish.tmpl"}

// subcommands go straight to the real CLI rather than starting a session.
var subcommands = []string{"agents", "attach", "logs", "stop", "rm", "mcp", "plugin", "config", "update", "doctor", "auth", "login", "logout", "install", "setup-token", "remote-control"}

// Script returns the integration for shell, which may be a name or a path
// such as $SHELL holds.
func Script(shell string) (string, error) {
	file, ok := files[filepath.Base(shell)]
	if !ok {
		return "", fmt.Errorf("no shell integration for %q: botropolis has one for %s", shell, strings.Join(Shells, ", "))
	}
	tmpl, err := template.New(file).Funcs(template.FuncMap{"join": strings.Join}).ParseFS(templates, file)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, struct{ Subcommands []string }{subcommands}); err != nil {
		return "", err
	}
	return out.String(), nil
}
