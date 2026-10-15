# 80. Botropolis does not run on macOS

**Open. Filed 2026-10-15 (r393) at Aria's request: "We need to also support macOS". Decided by Aria the same day; nobody's call now — what is left is cutting 0.1.6, the tap's first publish, and the website's install tab.**

Aria asked for it in those words.
Nothing on the roadmap mentioned macOS or darwin before this item.

**It already builds, which is the part one might expect to be hard.**
`CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...` succeeds, as does `amd64`, and `go vet` with the same environment is clean, test files included.
Ebitengine reaches Cocoa and Metal through purego on macOS, so the Makefile's `export CGO_ENABLED = 0` does not stand in the way and a Linux runner can cross-compile it.
What stands in the way is everything the binary assumes about the machine once it is running, and every one of those fails quietly or with a Linux-shaped message rather than refusing to start.

## What assumes Linux

| where | what it does | on macOS |
| --- | --- | --- |
| `pkg/state/load.go`, `UnixSocketConnected` | reads `/proc/net/unix` to see whether anyone holds a background job's `ptySock` | the open fails and it returns `false`, **always and silently** |
| `pkg/control/control.go`, `Reveal` | `xdg-open` | not installed; `open` is the equivalent |
| `pkg/control/control.go`, `clipboards` | `wl-copy`, `xclip`, `xsel` | none installed; `pbcopy` is always there |
| `pkg/control/terminal.go`, `knownTerminals` | eight Linux terminals, launched as `<terminal> -e <argv>` | kitty, alacritty and wezterm exist; Terminal.app and iTerm2 are launched with `open -a` or `osascript`, not argv |
| `pkg/commands/notify.go`, `NotifySend` | `notify-send` | not installed; `osascript -e 'display notification …'` is the stock route |
| `pkg/commands/doctor.go` | advises `systemctl --user enable --now botropolisd` | no systemd; the daemon wants a launchd agent in `~/Library/LaunchAgents` |
| `packaging/` | `.deb`, `.rpm`, Arch, `botropolisd.service`, `botropolis.desktop` | none apply |
| `.github/workflows/*.yml` | every job `runs-on: ubuntu-latest` | the suite has never run on darwin |

**The first row is the one that matters, because it is wrong rather than missing.**
`state.busy` returns `Unattended` for a background session that nobody is attached to, and the attached test is that `/proc` read.
On macOS every working background session would read as unattended, whoever is watching it, and nothing would say why.
The other rows fail loudly when pressed — a missing `xdg-open` is an error on the card — but the city would be quietly drawing the wrong state.
macOS has no `/proc`; the candidates are `lsof -U` per socket, which is slow at one call a session a tick, or `proc_pidfdinfo` through libproc, which needs purego or cgo.
And it has to be checked first that Claude Code on macOS writes the same roster with a `ptySock` at all: the fixtures in `pkg/claude/roster_test.go` are all Linux paths under `/tmp/cc-daemon-<uid>/`, and nobody has looked at a Mac's.

**Things that already work and need no change:** `ProcessAlive` is `kill(pid, 0)`, which is POSIX; `proto.SocketPath` falls back to `os.TempDir()` when `XDG_RUNTIME_DIR` is unset, which on macOS is the per-user `$TMPDIR`; the config, layout and usage-probe paths fall back to `~/.config` and `~/.local/state`, which are unidiomatic on a Mac but correct.

**Out of scope for the shipped program:** `tools/measure-render` reads `/proc/<pid>/stat` and `smaps_rollup`, and `pkg/demo` reads `/proc/<pid>/cmdline` and films in a Linux container.
They are development tools and can stay Linux-only, but item 69's CPU figure and items 61, 66 and 70's memory figures were measured with them, so none of those numbers transfers to a Mac.

## Decided, 2026-10-15

Aria: *"We are already hosting deb and dnf registries in ../botropolis.city. Let's just add the tap there like we do for the linux packages. I don't have a mac, so best we can do for now is the ci and tap."*

So the tap is served from `pkg.botropolis.city/brew` beside the APT and RPM repositories, and the formula builds from source, so nothing needs Apple signing.
Verification is the macOS CI runner and the tap's install test on it, and nothing more: nobody has opened the window on a Mac.

## What landed

| commit | what |
| --- | --- |
| `3590caa` | the attached probe reads `netstat -f unix -n` on macOS, one file per OS behind `UnixSocketConnected` |
| `3e86480` | `control.Desktop`: `open`, `pbcopy`, Terminal.app over `osascript`; notifications over AppleScript; `doctor` per OS |
| `ceb0bd1` | `test-macos` in CI, and `darwin-check` in `make lint` |
| `345bf5c` | the tap: `tools/build-brew-tap`, `packaging/brew/`, the `packages` workflow's `tap`, `tap-macos` and `publish-tap` jobs |
| `571c2a3` | macOS jobs on Apple silicon (`macos-26`) and Intel (`macos-26-intel`); `dry_run` on `packages` |
| `cb11e28` | `botropolis shell-init` for bash, zsh and fish |
| `a507be5` | the socket path on macOS; the daemon started under launchd in the macOS brew test |

**The netstat reading is proven, not inferred.**
The parser rested on BSD copying a listener's path onto each socket it accepts, which nobody here could check.
`TestUnixSocketConnected` opens a real listener and dials it, and it passed on `macos-latest` (arm64) in run 38693896779.

**The tap is proven on both kinds of Mac before any release.**
A `packages` dry run (38695743320) built the tap and installed from it on `macos-26` ("brew on Darwin arm64 installs botropolis 0.1.5") and `macos-26-intel` ("Darwin x86_64"), with `brew test` passing on each.
Before that, on Linux Homebrew:
in `homebrew/brew:4.6.20` against a static server, `brew tap`, `brew install` (a source build), `brew test`, and `brew update` moving an existing tap from 0.1.4 to 0.1.5.
R2 ignores the `?service=` query git opens with, so git falls back to its static protocol, checked against the live host.

**The shell integration was wrong on a Mac twice over, and is now per shell everywhere.**
A side session caught it: the instructions said to source `/usr/share/botropolis/botropolis.bash` from `~/.bashrc`, but Homebrew installs under its own prefix, and macOS's default shell is zsh, which never reads `~/.bashrc`.
Aria widened it: *"We should account for and acknowledge that users could use many different shells and we should support the popular ones on any system."*
`botropolis shell-init bash|zsh|fish` (`cb11e28`) prints the script, so the rc line names no path; the old file is a one-line shim to it.
The integration test runs each shell in a real pty, on Linux in CI and on macOS's own bash 3.2 and zsh, and fails rather than skips a missing shell under CI.
Other shells are named as unsupported rather than silently ignored.

**The daemon under launchd, which nothing had started until `a507be5`.**
A second side session asked whether `botropolisd`, run by `brew services`, can find what it runs, since launchd's PATH is `/usr/bin:/bin:/usr/sbin:/sbin`.
It can: the daemon's only bare-name exec is `netstat`, in `/usr/sbin`, and every `claude` exec is in the city's own process, which the user starts from a shell.
`netstat` is absolute now regardless, and the service sets Homebrew's `std_service_path_env`.
The question's shape found a real fault next to it: **the socket path came from `XDG_RUNTIME_DIR`, else `TMPDIR`, and launchd gives the daemon neither of the shell's**, so the daemon could listen in one place while every client dialled another.
On macOS it is now `~/Library/Caches/botropolis/botropolis.sock`, which depends on `HOME` alone, the one variable launchd and the shell agree on.
The macOS brew test now starts the service and requires `botropolis doctor` to reach the daemon, and a `packages` dry run builds the tap from the commit under test rather than the last release, so that test runs the code about to ship.
`XDG_CONFIG_HOME` has the same shape and is left: a Mac user who sets it in their shell gets a daemon reading `~/.config` instead, which is a narrower case than every user, and is written here so it is not rediscovered.

**Found on the way, and fixed:** the settings picker stores a terminal as `BOTROPOLIS_TERMINAL`, which was used verbatim, so picking alacritty on Linux ran it without `-e`. A lone known name there now gets its convention.

## Still open

1. **Cut 0.1.6.** No release contains the fixes above; a formula for 0.1.5 would build a city that reads every background session as unattended on a Mac.
2. **The tap's first publish**: run `packages` by hand with `new_tap`. The automatic run after the release will fail its `tap` job, by design, because there is no published history to continue; APT and RPM publish regardless.
3. **The website**: a macOS tab in `src/data/install.ts` and a tap check in `testing/acceptance/verify.sh`, after the publish, so neither points at a 404. The README's install section gains the same two lines then.
4. ~~**Unverified still:** that Claude Code on macOS writes a roster with a `ptySock` at all.~~ Answered below.

## What Claude Code does on macOS, 2026-10-15

Read from the strings of the 2.1.300 binary on Aria's desk (`strings -n 6 /opt/claude-code/bin/claude`), which is Bun-compiled and so carries its JavaScript as text.
The function names below are minified and will not survive a release; the logic is what matters.

- **The roster and the pty sockets are the same on macOS as on Linux.** The reaper picks pid files or sockets with `O()==="windows" ? [pid dir, ".pid"] : [pty dir, ".sock"]`; there is no macOS branch, so `ptySock` is written there exactly as here.
- **The socket directory is `/tmp/cc-daemon-<uid>/<8 hex>/pty`, hardcoded**, not `os.tmpdir()` — only Termux differs. So on a Mac it is not under `/var/folders/...`, and it is the string handed to `bind()`, which is what `netstat` prints even though `/tmp` is a symlink to `/private/tmp` there. The probe's comparison is string-for-string, so that matters, and it holds.
- **The roster is `<config dir>/daemon/roster.json` and sessions are `<config dir>/sessions/`**, both from the one config-dir function that honours `CLAUDE_CONFIG_DIR`, on every platform; the loader reads them from `~/.claude` already.
- **Background sessions are supported on macOS, not incidental to it.** The daemon acquires its controlling tty through `/usr/lib/libSystem.B.dylib` on macOS and `libc.so.6` on Linux, and its install telemetry carries `platform_darwin` beside `platform_linux`.
- **One thing that differs, and does not touch us:** inside Claude Code's own sandbox, `open` and `osascript` are blocked on macOS unless `allowAppleEvents` is set. That is Claude's tool sandbox; Botropolis calls them from its own process.

Public notes agree: background sessions arrived around 2.1.139 as a research preview, a 2.1.142 fix stopped sessions disappearing across macOS sleep, and one report has backgrounding failing in Terminal.app after 2.1.141.
Those are third-party pages and issue reports rather than Anthropic's documentation, which says only what `--bg`, `attach` and `agents` do.

- https://code.claude.com/docs/en/cli-usage
- https://newreleases.io/project/github/CometixSpace/claude-code/release/v2.1.142
- https://claudeissues.com/issue/60616-bug-cant-background-sessions

## What the fix touched, as filed

- `pkg/state/load.go`: an attached probe per platform behind the existing `Probes.Attached` seam, which `pkg/appd/daemon.go:81` already wires, so nothing above it moves. `load_test.go` writes a `/proc/net/unix`-shaped file and needs a darwin counterpart.
- `pkg/control`: `Reveal`, `clipboards` and `knownTerminals` chosen by `runtime.GOOS`, with Terminal.app and iTerm2 added. `KnownTerminals` is quoted by `doctor` and by the error text in `errNoTerminal`.
- `pkg/commands/notify.go`: a second `Notifier`.
- `pkg/commands/doctor.go`: the daemon advice per platform.
- `.github/workflows/ci.yml`: a `macos-latest` job running `make test-unit`, plus darwin in the build matrix.
- `release.yml` and a new tap repository for the formula. `tools/release-aliases`' fixed names, which botropolis.city links, are unaffected because they name Linux packages.
- `README.md` and the website's install section gain a `brew` line.
