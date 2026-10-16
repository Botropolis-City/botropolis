# Changelog

All notable changes to Botropolis are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **The footer names the session that has waited longest for you.**
  While a session needs you, a line above the key row reads `needs-you: <its title>`, with `(+N more)` when others wait too, in the order triage walks them.
  A click on the line attaches to that session; a click on a building still only selects it.
  A status message draws above the line rather than over it.

- **Groups: sessions across directories that are one piece of work.**
  Botropolis groups sessions by what they already say and how they were started: a shared ticket key, pull-request link or `Depends-On:` line, agent teams and sessions that message each other, the same pasted text, rare words in common, a shared branch key or files, and sessions started or continued close together.
  Nothing is tagged: each join says why, a guess is marked as one, and prompts are read where Claude Code keeps them and never copied.
  A member wears a badge with its group's short name and a dot in the tone of whose move it is (yours, the agents', someone else's, or nobody's), and its card names the group and why it joined.
  Each group is drawn as a line across its districts, with a station in each in the order its work started; a group lines setting hides them.
  `g`, or a click on City Hall, opens a board of every group with what happened last time, what is next, whose move it is and how long it has waited, and Enter focuses the map on one.
  Move to group on a card, or dragging the selected building onto a station, moves a session between groups, and the correction is remembered.
  The first launch proposes the groups found in the last 30 days on one screen, where they can be kept, merged or split, until you confirm or ask to be asked later.
  `Shift+N`, or the board's button, opens New session: a directory completed with Tab, a prompt, and the group guessed from both, which the session starts in.
  Groups are kept in `~/.local/state/botropolis/groups.json`, for your own home only.

- **Drag a district to another slot.**
  Press on a district's floor or name sign and drag it; the slot it would land on is outlined, and it stays where you drop it.
  Dropping it on another district swaps the two, and the ring just outside the city takes it too.
  Hold Shift or Alt to pan instead, wherever you press.

### Changed

- **Trees are scenery, and look it.**
  The woods between districts grow in natural clusters with open ground between them instead of one tree to a cell, they are drawn quieter than buildings and everything else you can hover, and the ground under them no longer shows a card.
- **The city stands on an island.**
  The map no longer ends at a cut: the plan stands on land with a soft, irregular coast, a beach and shallows, in a sea with a faint far shore, and the island grows as the city does.
  The river runs out to the sea at both ends between banks that wander like a river's, opening into the sea and fading into its shallows, and arriving and departing sessions' tugs sail in and out from the sea.
  You can pan far enough to see the coast all round.
- **Roads are laid only where two districts are connected.**
  The ground between districts is grass, and a road appears on it when a session reads or edits files in another project, messages a teammate working there, or has a subagent work there.
  Each pair of districts has one road, whichever way it was crossed, and its card shows the traffic both ways.
  The road stays while any session that made the connection is on the map, and grows over when the last one goes.
  Lamps and street trees stand only along roads.
- **A car is one session working across a connection**, so the number of cars on a road is the number of sessions working across it right now, and a road that nobody is working on has no cars.
- **A stopped session stays on its own lot, and the storage yard is gone.**
  A session whose process has stopped is *resting*; after `shuttered_days` (7) quiet it is *shuttered*, and after `overgrown_days` (30) it is *overgrown*.
  Its card names the rung and says how long it has been quiet, and a district stays on the map while it has any session at all.
  The state once called `parked` is now `resting` everywhere, in `botropolis status --all`, its JSON and the TUI.
- **Nothing leaves the map on a timer.**
  Every stopped session Claude Code still has a transcript for is shown, however old.
  The daemon keeps a ledger of the sessions it has shown — ids, directories and times only, in `~/.local/state/botropolis/sessions.json` —
  so a session whose transcript Claude Code has deleted keeps its lot, titled from your own prompts, and its card says the conversation is gone.
- **Each rung is drawn.**
  A stopped building's windows go dark, with no beacon or sign; a shuttered one has planks nailed across each of its own windows; and its lot ages: the paving wears and cracks, and an overgrown lot grows grass in the cracks, tufts at the base of the walls and in the yard, and now and then a sapling.
  When an overgrown session wakes, its lot is mowed in 1.5 s, or cross-faded in 0.3 s under reduced motion.
- **Codex rollouts are read through a 64 KB head and tail**, as Claude's transcripts already were, so a long Codex history loads in a time bounded by the number of rollouts rather than their size.

### Removed

- **`parked_days`**, its flag and its settings row: nothing is taken off the map by age any more.
  A config file that still sets it loads as before, and `shuttered_days` and `overgrown_days` replace it in the settings panel.

## [0.1.7] - 2026-10-15

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

[Unreleased]: https://github.com/Botropolis-City/botropolis/compare/v0.1.7...HEAD
[0.1.7]: https://github.com/Botropolis-City/botropolis/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/Botropolis-City/botropolis/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/Botropolis-City/botropolis/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/Botropolis-City/botropolis/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/Botropolis-City/botropolis/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/Botropolis-City/botropolis/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Botropolis-City/botropolis/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Botropolis-City/botropolis/releases/tag/v0.1.0
