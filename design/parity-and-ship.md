## Parity

Roadmap phase 10, 2026-09-18.
A session's title is painted on its building: a fascia over the door when it is short enough to read there, up the flank of a tall one when it is not, and a rooftop billboard of two ellipsised lines otherwise.
Nothing is hung below seven pixels, and the name plate appears only on hover — the same rule the towers follow.

A selected building's card pins beside it and follows it with its actions — attach or resume, stop, a new session in its directory (also `c`),
reveal the folder, copy the path, hide the project, star it — all through `pkg/control` and the `claude` CLI except hide and star,
which are marks in `layout.json` and never touch `~/.claude`.
The sidebar (`b`) lists projects starred first with their counts, sessions by urgency with resting last, and the hidden projects to show again;
the card docks at its foot while it is open.
A district's name plate shows only while something in it is awake, or on hover or selection.
Night follows the local clock (21:00 to 06:00), `[` and `]` scrub it an hour at a time and `n` cycles night, day and live;
an unattended session no longer makes it night — a lit lamp at night is what says a session is awake.
The transcript's `pr` action records give each PR a state; a merged PR's flag turns green and the building shows off for two and a half seconds
the first time the map sees the merge, never on start-up.
API errors keep their smoke.

## Beyond parity

Roadmap phase 11, 2026-09-18.
Each session carries a week of usage by unix hour and the snapshot carries the teams, merged through the harness.
`City.Breakdown` adds the spend up over the last hour, day or week by model, project and session
(each session's lifetime cost pro-rated by the window's share of its tokens, an estimate shown as one);
`Series` gives tokens per hour for the city, a district or a session, drawn as sparklines under the cards and in the plant's panel,
which opens on the plant or `x`.
`city.Log` turns the stream of snapshots into events by what changed between one and the next; `t` lists them newest first and jumps,
and when the window comes back into focus the same panel shows what needed you or went wrong meanwhile.
`/` or `Ctrl+K` (`⌘K` on macOS) opens the finder, a panel over the city on the overlay stack (item 82), which finds sessions by what was said in them while the map dims every session it did not find;
`Enter` attaches a live session and resumes a stopped one exactly as the card's buttons do, or offers a new session where a deleted conversation was, started as the card's *new session here* starts one,
`Ctrl+Y` copies the command that resumes it, `Ctrl+M` shows it on the map, and `Esc` clears the map's filter.
The three chords are settings, `find_key`, `find_copy_key` and `find_show_key`, written as modifiers and a key such as `alt+s`, where `mod` is ⌘ on macOS and Ctrl elsewhere;
showing on the map stays Control+M on macOS, and a chord macOS keeps for itself, such as ⌘M, which minimizes the window, is refused when the city starts.
`botropolis find` searches what was said in every session — your prompts, Claude's replies and recaps, the commands and paths it touched, and titles, with tool output only on `--tool-output` — from the daemon's in-memory index, or from one of its own when the daemon is down, written nowhere either way (item 82).
There is one city: it listens on a socket of its own beside the daemon's, so a second `botropolis city` brings the first forward and exits, and `botropolis find` with no words opens the finder in it, or becomes the city with the finder open; `botropolis find --help` shows how to bind that to a key in GNOME, KDE, macOS and i3.
Bringing the window forward goes behind a seam item 84 fills in; until then the window is restored if it was minimized and asks for attention.
A team is a camp: its lead's building tied to its members' with dashed lines, from the rosters and the sessions' own team names, with a card and a line on each member's card.
`daily_budget_usd` measures the strip's cost chip and the plant's card against a daily target, accent-toned from 80 % and error-toned beyond it.

## Ship

Roadmap phase 12, 2026-09-18.
`botropolis doctor` checks the daemon, the hooks, a terminal, the `claude` CLI, the harness homes and the display, with the command to run for anything short of ok;
a Wayland session is noted as XWayland, which is how Ebitengine (GLFW/X11) runs there today.
`--headless` re-runs the city under `xvfb-run` on a 1100×760 virtual display with the desktop's displays hidden from it, so screenshots, `--keys` and `--record` never open a window;
the acceptance test shoots that way and skips only when there is no `xvfb-run`.
`--record dir --seconds n` writes ten frames a second and presses `--keys` two seconds apart; `make gif` turns twenty-four seconds of that into `docs/botropolis.gif` through ffmpeg.
The README leads with the latest screenshot, the GIF, the four commands of a first run and the shell helper.
Releasing is manual and publishes what CI already built.
The version is one line in `VERSION`, read by the Makefile's ldflags, by `packaging/nfpm.yaml`, and by the release workflow;
`make version-check` fails, naming files, if `VERSION`, `CHANGELOG.md` and the built binaries disagree.
CI builds the three binaries on every push to `main` and packages them as a tarball, a `.deb`, an `.rpm` and an Arch package from one nfpm config, with `SHA256SUMS`, and uploads them as a run artifact.
The release workflow is `workflow_dispatch` only: it reads `VERSION`, refuses a version already tagged, downloads that commit's artifact rather than rebuilding, takes its notes from the changelog section for that version, and creates the tag and the release together with `--target`, so the tag necessarily points at the commit that was tested.
The `botropolis-git` VCS package it used to ship from is retired; nfpm's archlinux packager produces the Arch package now.
