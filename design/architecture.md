## Architecture

```
~/.claude ──inotify──▶ botropolisd ◀── botropolis-hook (stdin JSON → socket, always exit 0)
                          │
                          │ $XDG_RUNTIME_DIR/botropolis/botropolis.sock
                          │ JSON lines: {"op":"snapshot"} / {"op":"subscribe"} / {"op":"action",...}
                          ▼
              ┌───────────┴───────────┐
          botropolis (Ebitengine)   botropolis status / --tui / waybar
```

- **`botropolisd`** owns the model.
  It scans once at start, then updates from inotify events and hook pushes.
  It serves snapshots and a subscription stream, and executes actions by shelling out to `claude`.
- **`botropolis-hook`** is installed into `~/.claude/settings.json` hooks by `botropolis install-hooks`.
  It forwards the hook's JSON to the socket with a short timeout and exits 0 no matter what,
  so a dead daemon never slows a session.
- **`botropolis`** is the client.
  `botropolis status` prints a table; `botropolis` with no arguments opens the city.

**Every request and response carries its end's release and protocol** (item 149).
An upgrade leaves the old daemon running until it is restarted,
and running the two out of sync could go wrong in ways nobody tested,
so any difference is refused (item 152), except for hook events, which never fail a tool call.
A command says so on one line, naming both and how to restart the daemon.
The map draws no city: a dialog names both, with **Exit** and **Restart Daemon**,
or **Restart Botropolis** when the daemon is the newer one, which runs the map again from its path on disk.
`pkg/daemonctl` restarts the daemon the way it was started, acting only on the process the socket names as its peer:
its systemd user unit, its Homebrew launchd job, or that one process stopped and this build's `botropolisd` started with its arguments.
`proto.Protocol` rises only when one end would misread the other, never for an added field.

**Every state file carries a version and the oldest version that can read it**, as `stats.db` does (item 116).
An older file is migrated forward, a newer one this build can read keeps its version when saved,
and one it cannot read is refused by name and never written over: a reset would lose what the person arranged.

Layout (per the Go conventions in this workspace):

```
cmd/botropolis/         client entry point
cmd/botropolisd/        daemon entry point
cmd/botropolis-hook/    hook forwarder
pkg/app/                fx wiring: the botropolis CLI graph and the botropolisd lifecycle
pkg/cli/                cobra commands, one constructor per command, config loaded inside RunE
pkg/commands/           what the commands do (status table, hooks, session control), tested with fakes
pkg/config/             viper: flags, BOTROPOLIS_* env, ~/.config/botropolis/config.{toml,yaml,json}
pkg/city/               the map model: districts, buildings, camera, sticky layout, hover cards
pkg/render/             the Ebitengine window that draws pkg/city and forwards clicks
pkg/format/             the shared number formatting the table and the cards both use
pkg/claude/             read-only model of ~/.claude (transcripts, sessions, subagents, stats, mcp config)
pkg/state/              the city model: districts, buildings, workers, gauges, derived states
pkg/control/            actions via the claude CLI; terminal spawning
pkg/proto/              socket protocol
pkg/daemon/             watcher + server wiring
pkg/city/               rendering (Ebitengine), layout persistence, input
testing/{integration,acceptance,helpers}/
tools/                  analyze-history.py and the fixture generator
```

Paths follow XDG: config in `~/.config/botropolis`, the saved layout in `~/.local/state/botropolis`,
the socket under `$XDG_RUNTIME_DIR/botropolis`.
Every store finds its state directory through `pkg/statedir`.
A city run with `--headless`, `--record`, `--script` or `--home` works in a temporary one instead,
seeded from a copy of `<home>/.local/state/botropolis` when it was given a home,
and it starts no `claude -p` (item 111).
