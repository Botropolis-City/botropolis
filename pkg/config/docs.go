package config

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// KeyDoc is one config key as botropolis-config(5) describes it. Item 167.
type KeyDoc struct {
	Key     string
	Help    string
	Default string
	Env     string
}

// unflagged describes the keys that never had a flag; every other key's
// help is its flag's, so the two cannot drift apart.
var unflagged = map[string]string{
	KeyTerminal:       "the command that opens a terminal for attach, as a prefix such as \"wezterm start --\" (default: found from $TERMINAL and what is installed)",
	KeyHookCommand:    "the command install writes into Claude Code's hooks",
	KeyCodexHome:      "the directory holding Codex's sessions (default: ~/.codex)",
	KeyGlobalShortcut: "a chord that opens the finder from any application, such as mod+shift+k; empty for none",
	KeyGoneSessions:   "keep sessions whose transcript Claude Code deleted on the map; their statistics are kept either way",
	KeyScriptedRuns:   "hidden or shown: the sessions a program started, such as claude -p and the Agent SDK; hidden, they are off the map and out of the finder, and their tokens and spend still count",
}

// Docs are every config key, sorted, with its help, default and
// environment variable.
func Docs() []KeyDoc {
	v := NewViper()
	v.SetEnvPrefix("botropolis_docs_reads_no_environment")
	flags := pflag.NewFlagSet("docs", pflag.ContinueOnError)
	BindFlags(v, flags)
	var docs []KeyDoc
	for _, key := range Keys(v) {
		help, value := unflagged[key], ""
		if d := v.Get(key); d != nil {
			value = fmt.Sprint(d)
		}
		if f := flags.Lookup(key); f != nil {
			_, help = pflag.UnquoteUsage(f)
			value = f.DefValue
		}
		docs = append(docs, KeyDoc{Key: key, Help: help, Default: value, Env: "BOTROPOLIS_" + strings.ToUpper(key)})
	}
	return docs
}
