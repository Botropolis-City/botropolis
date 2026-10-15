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

**The netstat reading is proven, not inferred.**
The parser rested on BSD copying a listener's path onto each socket it accepts, which nobody here could check.
`TestUnixSocketConnected` opens a real listener and dials it, and it passed on `macos-latest` (arm64) in run 38693896779.

**The tap is proven on Linux Homebrew, and the macOS half runs at publish.**
In `homebrew/brew:4.6.20` against a static server: `brew tap`, `brew install` (a source build), `brew test`, and `brew update` moving an existing tap from 0.1.4 to 0.1.5.
R2 ignores the `?service=` query git opens with, so git falls back to its static protocol, checked against the live host.

**Found on the way, and fixed:** the settings picker stores a terminal as `BOTROPOLIS_TERMINAL`, which was used verbatim, so picking alacritty on Linux ran it without `-e`. A lone known name there now gets its convention.

## Still open

1. **Cut 0.1.6.** No release contains the fixes above; a formula for 0.1.5 would build a city that reads every background session as unattended on a Mac.
2. **The tap's first publish**: run `packages` by hand with `new_tap`. The automatic run after the release will fail its `tap` job, by design, because there is no published history to continue; APT and RPM publish regardless.
3. **The website**: a macOS tab in `src/data/install.ts` and a tap check in `testing/acceptance/verify.sh`, after the publish, so neither points at a 404. The README's install section gains the same two lines then.
4. **Unverified still:** that Claude Code on macOS writes a roster with a `ptySock` at all. Every fixture is a Linux path, and no Mac here has run it. If it does not, the probe is never asked and background sessions read as unattended for a different reason.

## What the fix touched, as filed

- `pkg/state/load.go`: an attached probe per platform behind the existing `Probes.Attached` seam, which `pkg/appd/daemon.go:81` already wires, so nothing above it moves. `load_test.go` writes a `/proc/net/unix`-shaped file and needs a darwin counterpart.
- `pkg/control`: `Reveal`, `clipboards` and `knownTerminals` chosen by `runtime.GOOS`, with Terminal.app and iTerm2 added. `KnownTerminals` is quoted by `doctor` and by the error text in `errNoTerminal`.
- `pkg/commands/notify.go`: a second `Notifier`.
- `pkg/commands/doctor.go`: the daemon advice per platform.
- `.github/workflows/ci.yml`: a `macos-latest` job running `make test-unit`, plus darwin in the build matrix.
- `release.yml` and a new tap repository for the formula. `tools/release-aliases`' fixed names, which botropolis.city links, are unaffected because they name Linux packages.
- `README.md` and the website's install section gain a `brew` line.
