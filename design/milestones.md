## Milestones

Each milestone ends with tests green, `make lint` and `make format` clean, and a signed commit.

### 0 — Scaffold

- `go.mod`, Makefile from the workspace template, `cmd/` and `pkg/` skeleton, `.gitignore`.
- `botropolis version`.
- CI: GitHub Actions running `make test lint`.

### 1 — Read model (`pkg/claude`), TDD

- Fixture generator: `tools/make-fixtures.py` copies a scrubbed slice of `~/.claude` into `testing/helpers/fixtures/`
  (prompts and tool output replaced, structure and `usage` kept).
- Parse: live session records, transcript head (metadata) and tail (turn state), `usage` per message, subagent trees, `stats-cache.json`, MCP config.
- Filter the 267-byte `bridge-session` stubs.
- `botropolis status` prints a table straight from the read model — no daemon yet.
- **Done when** the table matches the five live sessions on this machine: state, context %, tokens/hour, needs-you.

### 2 — Daemon and socket

- `pkg/daemon`: initial scan, `fsnotify` on `~/.claude/{sessions,projects}`, debounce, incremental re-read of changed tails.
- `pkg/proto`: snapshot, subscribe, action.
- `botropolis-hook` and `botropolis install-hooks`.
- `botropolis status` now reads from the socket and falls back to a direct scan when the daemon is down.
- systemd user unit and a PKGBUILD under `~/workspaces/aur/botropolis-git`.
- **Done when** a `PreToolUse` hook shows up in `botropolis status` in under 100 ms and the daemon idles under 20 MB.
  Met on 2026-09-17 (r38): 6 ms hook-to-status; 20.1 MB RSS (9.9 MB anonymous, 8.3 MB file-backed, from `/proc/<pid>/smaps_rollup`)
  with the parked catalogue loaded for 184 transcripts.
  Getting there needed the daemon wiring split into `pkg/appd` so `botropolisd` no longer links Ebitengine and bubbletea through `pkg/app`,
  and a 16 MiB Go heap cap (`GOMEMLIMIT` overrides it).
  Re-met at r48 after the city hall, file-touch roads and line cards pushed it to 21.4 MB: 18.2 MB RSS (7.9 MB anonymous, 8.5 MB file-backed).
  The live heap after a scan is about 1 MB; the rest was scan garbage the scavenger had not returned yet,
  so the daemon now calls `debug.FreeOSMemory` after every rescan (hook events never rescan, so the 6 ms path is untouched).
  Checked again on 2026-09-18 after the hourly usage buckets and the isometric city: 14.7 MB RSS (4.8 MB anonymous) with 75 sessions;
  `botropolis-hook` (cobra+viper, 12 MB on disk) starts, parses and delivers in 2.6 ms median, so its dependencies are not worth trimming.
  The catalogue had pushed an unsplit r35 daemon to 36 MB.
  Both the cap and the per-rescan release were undone on 2026-10-21 (item 157): by 0.2.2 the collector was most of the daemon's CPU,
  and the budget had become 64 MiB settled, spent on caches that save CPU rather than on a tight heap.

### 3 — Control

- `pkg/control`: `new`, `attach`, `stop`, `resume`, `rm`, each a thin wrapper over the `claude` CLI.
- Terminal spawning: `BOTROPOLIS_TERMINAL`, else `$TERMINAL`, else a known list.
- A shell function for `~/.bashrc` so `claude` from a shell becomes `claude --bg` + `claude attach`.
- **Done when** a session can be started from `botropolis new <dir>`, its terminal closed, and reopened from `botropolis attach` with the conversation intact.

### 4 — City, first pass

- `pkg/city`: districts as rectangles, buildings as blocks, fill bars for context, pulse for needs-you, hover cards, click actions.
- Sticky layout persisted to `~/.local/state/botropolis/layout.json`.
- Camera: pan, zoom.
- **Done when** every table row from `botropolis status` is a building and every building's hover card shows the same numbers.

### 5 — City, second pass

- Sprites, power plant and lines, radio towers and beams, cranes, roads between districts, day/night.
- Demolish and prune flows.

### 6 — Satellites

- `--tui` client, waybar module, desktop notification on needs-you.
- Second harness adapter (Codex) to prove the seam.
