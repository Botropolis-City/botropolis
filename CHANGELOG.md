# Changelog

All notable changes to Botropolis are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Shell completion for bash, zsh and fish**, installed by every package where each shell looks for it:
  the `.deb`, `.rpm` and Arch packages, the tarball's `completions/`, and Homebrew.
  Session commands complete what they can act on, with each session's title:
  job ids for `attach` and `stop` (live background sessions) and `rm` (parked ones too),
  session ids for `resume` (parked sessions), and directories only for `new`.
  bash completion needs the bash-completion package, as every bash completion does.

## [0.1.6] - 2026-10-15

### Added

- **macOS, on Apple silicon and on Intel, from a Homebrew tap at pkg.botropolis.city.**
  `brew tap botropolis-city/botropolis https://pkg.botropolis.city/brew`, then `brew install botropolis-city/botropolis/botropolis`;
  `brew services start botropolis` runs the daemon.
  The formula builds from the release's source, and every release is install-tested on both kinds of Mac before the tap is published.
  Nobody has yet looked at the window on a Mac, so tell us what you see.
- **Shell integration for zsh and fish, as well as bash**, from `botropolis shell-init <shell>`:
  `eval "$(botropolis shell-init zsh)"` in `~/.zshrc`, the same with `bash` in `~/.bashrc`,
  or `botropolis shell-init fish | source` in fish's `config.fish`.
  It names no install path, so the line is the same from every package and on every system.
- On macOS, reveal uses `open`, copy uses `pbcopy`, attach falls back to Terminal.app,
  notifications go through AppleScript, and `botropolis doctor` knows it is on a Mac.
- Every release carries its packages a second time under names without the version,
  so `https://github.com/Botropolis-City/botropolis/releases/latest/download/botropolis_amd64.deb` (and `.x86_64.rpm`, the tarballs and the Arch packages) is always the newest release.
  The install commands in the README and on botropolis.city use them.
- **APT and RPM repositories at pkg.botropolis.city.**
  Add the repository once and `apt upgrade` or `dnf upgrade` brings each new release.
  Both are signed; every release publishes to them as part of the release.
- `usage_refresh` sets how often the city fetches plan usage on its own: `manual`, or an interval such as `30m`.
  It is also a row in the settings panel, offering manual, 5m, 15m, 30m and 1h.
  The fetch spends no tokens but is a network call, so the default is `manual`.

### Changed

- `/usr/share/botropolis/botropolis.bash` now loads `botropolis shell-init bash`,
  so `~/.bashrc` files that source it keep working and get the same script.
- With `usage_refresh` at `manual`, the usage boats are off the river until `u` is pressed.
  They then sail in to their readings, stay a minute, and sail off.
  Pressing `u` again while they are there starts the minute over,
  and pressing it while they are sailing off sends them on round to come back in rather than turning them back.
- The usage boats drive to a new reading instead of jumping to it, and only ever forward.
  A reading that falls, such as a window turning over, sends the boat on past the limit and round from the start.

### Fixed

- Choosing a terminal in the settings panel now launches it the way it expects:
  picking alacritty, for example, used to run it without `-e`, so attach failed.

## [0.1.5] - 2026-10-10

### Changed

- **Botropolis has moved to [github.com/Botropolis-City/botropolis](https://github.com/Botropolis-City/botropolis), and has a website, [botropolis.city](https://botropolis.city).**
  The Go module is now `botropolis.city/botropolis`, so `go install botropolis.city/botropolis/cmd/botropolis@latest` works,
  and the module path stays put if the repository ever moves again.
  Links to the old repository redirect.
- The package homepage and the systemd units' documentation link point at botropolis.city.

### Added

- `--script FILE`, with `--record`, plays a choreography while the clip records:
  a pointer that glides to a session, a district or the power plant, clicks, zooms and pans, drawn into the frames.
  It is what films the website's clips.

### Fixed

- The usage boats follow Claude Code's figures while the city is open.
  They were read once at startup, so the river kept whatever the reading was when the city was opened;
  the city now re-reads the cache whenever Claude Code updates it.
- Recorded clips play at the speed the city runs.
  On a slow renderer every recorded frame advanced the animations three or four frames' worth, so clips ran fast and shook;
  while recording, time now advances by the frame rather than by the clock.
- Recording no longer stalls the city: frames are encoded off the game loop, and a frame on disk is always whole.

## [0.1.4] - 2026-10-09

### Added

- `--window WIDTHxHEIGHT` sizes the city's window, and under `--headless` the virtual display with it.
  `--fps` picks how many frames a second `--record` writes, and refuses a rate that does not divide 30.
  Both default to what they were, 1100x760 and ten.

### Fixed

- A plain `claude` from the shell integration no longer fails in a directory nobody has trusted yet.
  `claude --bg` cannot show the trust prompt,
  and the advice it printed — run `claude` there once — went straight back through the same wrapper.
  `botropolis new` now exits 3 for that refusal, and the wrapper runs that one session in the foreground so the prompt can appear.
  Later sessions in the directory go to the background as before.
- The usage boats and **generate summary** read from `--home` rather than the real home.
  A city pointed at other data showed the real account's limits,
  and a session sharing an id with a real one would summarise the real one.

## [0.1.3] - 2026-10-06

### Added

- **A summary on the building card.**
  The card's new **summary** button, or `i`, opens the recap Claude Code writes into a transcript when a session is left waiting.
  It is shown as written, under a line giving its age and how many turns the session has taken since.
  A session with no recap falls back to its last prompt and offers **generate summary**.
  That button runs your own `claude` CLI, so it uses your existing login and needs no API key.
  It runs Haiku with no tools, no MCP servers and no user settings, writes no transcript, and is fenced out of the city like the usage probe, so generating a summary never adds a building.
  `status --json` carries each session's `recap` and `lastPrompt`.

## [0.1.2] - 2026-09-30

### Added

- **aarch64 packages.** Every release now carries `.deb`, `.rpm`, Arch and tarball artefacts for both `amd64` and `arm64`.
  The Arch package had declared `aarch64` support since it was written and nothing had ever built it.

### Changed

- The release tarball now carries `packaging/` — both systemd user units, the desktop entry, the icon and the shell integration — so a package built from the tarball installs the same set as one built from source.
- Binaries are built with `CGO_ENABLED=0`.
  Ebitengine reaches GL and X11 through `dlopen`, so nothing needed cgo, and dropping it is what makes building for another architecture a plain cross-compile with no toolchain.

## [0.1.1] - 2026-09-30

### Fixed

- The `.deb` and `.rpm` packages no longer require `libXxf86vm` or ALSA.
  Neither is referenced by any binary — they came from the build headers CI installs, which is not the same as a runtime need, and requiring ALSA made every user install a sound stack for a program that never opens one.
- The packages now declare `hicolor-icon-theme`, which they need for the icon they install into that tree.

## [0.1.0] - 2026-09-30

First release.
Botropolis draws every Claude Code session on the machine as a city, and starts, opens, parks and wakes them through the `claude` CLI.

### Added

- `botropolis` — the isometric city.
  One district per project, one building per session, and every object stands for one datum you can read by hovering it.
  A building's height is its context used and its beacon is its state;
  rovers, drones, flags and smoke stand for turns in flight, subagents, pull requests and API errors.
- The plaza landmarks — the power plant for API spend, broken down by model, project and session over an hour, a day or a week;
  radio towers for MCP servers;
  the library for skills;
  and the city hall for Claude Code's own rollup.
- The river, the freight loop and the avenues — arrivals and departures, the day's tokens as one train per model, and traffic between projects whose sessions message each other or edit each other's files.
- Nine info views, a sidebar, a breakdown panel, a searchable list and a timeline that survives the window closing,
  including a "while you were away" summary of what needed you since the city was last seen.
- `botropolisd` — a daemon watching `~/.claude` by inotify, taking hook pushes, and serving the city model over a unix socket.
- `botropolis-hook` — the hook forwarder, installed into `~/.claude/settings.json` by `botropolis install-hooks` with a backup kept.
- Session management wrapping the `claude` CLI: `new`, `attach`, `stop`, `resume`, `rm` and `prune`.
  Botropolis never writes to `~/.claude` itself.
- `botropolis status`, `bar` for a status bar such as waybar, `tui` for the terminal table, `events` for the daemon log, and `doctor` for the install.
- `botropolis version`, and `--version` on all three binaries.
- Settings from flags, environment and `~/.config/botropolis/config.{toml,yaml,json}`, with an in-city settings panel that writes the file back.
- Headless rendering — `--screenshot` for one frame, `--record` for a sequence, and `--keys` to press keys first.
- `.deb`, `.rpm` and Arch packages, each installing the three binaries, both systemd user units, the desktop entry, the icon, the shell integration and the licences.

### Security

- The usage probe runs inside a fenced directory the session loader skips, and prunes its own transcripts, so reading usage cannot make Botropolis see itself as a session.

[Unreleased]: https://github.com/Botropolis-City/botropolis/compare/v0.1.6...HEAD
[0.1.6]: https://github.com/Botropolis-City/botropolis/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/Botropolis-City/botropolis/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/Botropolis-City/botropolis/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/Botropolis-City/botropolis/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/Botropolis-City/botropolis/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Botropolis-City/botropolis/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Botropolis-City/botropolis/releases/tag/v0.1.0
