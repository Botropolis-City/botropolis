# What 1.0.0 promises

Botropolis follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
From 1.0.0 on, nothing listed here is removed or renamed without a 2.0.0.
Adding is never breaking: a new command, flag, key, value or field can come in any release.

Each promise below is a golden file in [`testing/acceptance/testdata/surface/`](../testing/acceptance/testdata/surface),
which `TestSurface` regenerates from the code and compares, so a change to any of them fails until its golden file changes in the same commit.
`make version-check` then compares those files with the ones at the last release tag.
Anything gone since that release fails the check unless the changelog names it, in backticks, under a **Removed** heading written since then,
and, from 1.0.0 on, unless the major version rose.

These are Aria's rulings of 2026-10-20 (roadmap item 149).

## Promised

### The command line

Every command `botropolis --help` lists, and every flag each one shows, in the shape Aria chose for 1.0 (roadmap item 167):

- `new`, `attach`, `stop`, `rm`, `status`, `find`, `city`, `setup`, `doctor`, `events`, `bar`, `notify`, `shell-init` and `help`;
- the global flags `--config`, `--set key=value`, `--home`, `--socket` and `-h`/`--help`, and `-v`/`--version` on `botropolis` itself;
- `attach --dir --new-window`, `rm --older-than --dry-run`, `status -a/--all -l/--live --json --direct`, `find -n/--limit --tool-output`, `events --since`, `bar --format --watch` and `setup --dry-run --remove`.

Each flag's type and shorthand is promised with it.
[`commands.txt`](../testing/acceptance/testdata/surface/commands.txt) is walked from the root command of a release build.

Not promised: anything hidden or deprecated.
`tui`, `--tui`, `resume`, `prune`, `install-hooks`, `version` and the per-setting flags such as `--daily_budget_usd` keep working through 0.x, each naming its replacement, and are removed at 1.0.0.
`completion` is kept, hidden, for packagers.
The city's frame flags (`--screenshot`, `--record` and the rest) are only in builds with the `devtools` tag.

### Machine-readable output

- **`botropolis status --json`**: an array with one object per session.
  These keys and their JSON types are promised: `id`, `pid`, `harness`, `title`, `state`, `cwd`, `branch`, `model`, `contextPercent`, `startedAt`, `lastActivity` and `prs`,
  and in each of `prs`, `number`, `url`, `repository` and `state`.
  So are the values of `state`: `needs-you`, `working`, `waiting`, `unattended` and `resting`.
  Every other key in the object is the session model written whole, emitted today and free to change in any release.
  [`status-json.txt`](../testing/acceptance/testdata/surface/status-json.txt)
- **`botropolis bar`'s JSON**, which status bars parse: its keys `text`, `tooltip`, `class` and `percentage`, and the classes `needs-you`, `working` and `idle`.
  [`bar.txt`](../testing/acceptance/testdata/surface/bar.txt)

### The config file

Every key `config.toml` reads, with its type, and every named value a key accepts.
Defaults are not promised: the key bindings' keys are, but which key each one defaults to is the city's to change.
[`config.txt`](../testing/acceptance/testdata/surface/config.txt)

### Data files

`stats.db`, the layout (`layout.json`) and the session ledger (`sessions.json`) are **readable forward and never lost**.
Any later 1.x reads every file an earlier 1.x wrote, and migrates it.
A file written by a newer build that this one cannot read is refused by name, with a message to update, and never reset or written over.
The files' internal shape is not promised.
[`testing/acceptance/testdata/data/`](../testing/acceptance/testdata/data) holds a file of each version every build has written, and `TestSurface` loads each one;
[`data.txt`](../testing/acceptance/testdata/surface/data.txt) lists them.

### Installing

- the package name `botropolis` in the APT and RPM repositories at pkg.botropolis.city, and the Homebrew formula `botropolis`;
- the fixed release asset names, which botropolis.city's install commands link through `/releases/latest/download/`;
- the systemd user units `botropolisd.service` and `botropolis-notify.service`;
- the hook command `botropolis-hook`, which `botropolis install-hooks` writes into Claude Code's settings and which outlives an upgrade.

[`install.txt`](../testing/acceptance/testdata/surface/install.txt)

## Not promised

- **The daemon protocol** between `botropolis` and `botropolisd`.
  One package ships both ends, and a client refuses a daemon of another version and says which to restart (items 149 and 152).
- **The map's look and art**: what the city draws, and how.
- **The terminal table's layout** (`botropolis tui`).
- **The city's key bindings.**
  Their config keys are promised, so a binding you set keeps working; which key each one defaults to is not.
- **The AUR packages** `botropolis` and `botropolis-bin`.
  They are published to the AUR, not by this repository.
- Everything in `status --json` beyond the keys listed above.

## Removed before 1.0.0

- The `projection` config key, deprecated since the top-down view went (item 106).
  A config file that still sets it loads, and the key is ignored.
