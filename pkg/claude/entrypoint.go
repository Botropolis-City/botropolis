package claude

import "strings"

// Scripted reports whether a session was started by a program rather
// than by someone at a prompt: claude -p and the Agent SDK write an
// entrypoint starting sdk-. Anything else, unknown entrypoints included,
// is taken as interactive, so a session that cannot be told apart is
// never hidden. Item 273.
func Scripted(entrypoint string) bool {
	return strings.HasPrefix(entrypoint, "sdk-")
}
