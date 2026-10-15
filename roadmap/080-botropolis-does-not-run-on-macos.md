# 80. Botropolis does not run on macOS

**Open. Filed 2026-10-15 (r393) at Aria's request: "We need to also support macOS". Aria's call — how it is distributed, and what it is verified on.**

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

## The decision

**How it is installed.**
The release pipeline publishes packages, and the repo's rule is that installs go through a package manager, so a bare darwin tarball is not an install path.
On macOS that means Homebrew, and there are two shapes:
- **A formula in a tap**, `Botropolis-City/homebrew-tap`, that builds from the source tarball. Homebrew compiles it locally, so Gatekeeper never sees a downloaded binary and nothing needs signing.
- **A cask or formula that downloads a prebuilt binary.** Smaller to install, but an unsigned binary fetched over the network is quarantined, and signing and notarising it needs an Apple Developer Program membership and a CI secret for it.

The first costs nothing and fits the existing pipeline: one more step in `release.yml` that bumps the formula's version and checksum.
The daemon would ship as a `service do` block in the formula, which is how Homebrew writes a launchd agent, and `brew services start botropolis` replaces the `systemctl` line in `doctor`.

**What it is verified on.**
There is no Mac in the working arrangement, so nothing in this item can be looked at before it ships.
CI can run the suite on `macos-latest`, which covers the code paths but not the window, the Metal driver or how a frame looks.
Someone has to open the city on a Mac before it is called supported, and Aria judges frames.

## What the fix touches

- `pkg/state/load.go`: an attached probe per platform behind the existing `Probes.Attached` seam, which `pkg/appd/daemon.go:81` already wires, so nothing above it moves. `load_test.go` writes a `/proc/net/unix`-shaped file and needs a darwin counterpart.
- `pkg/control`: `Reveal`, `clipboards` and `knownTerminals` chosen by `runtime.GOOS`, with Terminal.app and iTerm2 added. `KnownTerminals` is quoted by `doctor` and by the error text in `errNoTerminal`.
- `pkg/commands/notify.go`: a second `Notifier`.
- `pkg/commands/doctor.go`: the daemon advice per platform.
- `.github/workflows/ci.yml`: a `macos-latest` job running `make test-unit`, plus darwin in the build matrix.
- `release.yml` and a new tap repository for the formula. `tools/release-aliases`' fixed names, which botropolis.city links, are unaffected because they name Linux packages.
- `README.md` and the website's install section gain a `brew` line.
