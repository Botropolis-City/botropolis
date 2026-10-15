# 81. Shell completion exists in the binary and is not packaged

**Done 2026-10-15 (`e677a69`, released as 0.1.7 at `62c50ee`). Filed 2026-10-15 (r406) at Aria's request: "shell completion for all the popular shells we support".**

Aria: *"shell completion for all the popular shells we support. I think our library has generation for this built in so we just need to build it into the packages."*

**The premise is right, and goes further than stated: the command already exists.**
Cobra adds a `completion` command to every root that does not disable it, and nothing here does (`grep -rn Completion pkg cmd` finds nothing), so on the current build:

```
$ bin/botropolis completion --help
  bash        Generate the autocompletion script for bash
  fish        Generate the autocompletion script for fish
  powershell  Generate the autocompletion script for powershell
  zsh         Generate the autocompletion script for zsh
```

`botropolisd` has one too, which is worth nothing — it takes no arguments — and `botropolis-hook` is not a cobra root and is never typed by a person.
So the shells to ship are the three `botropolis shell-init` supports (item 80, `cb11e28`): bash, zsh and fish.
PowerShell is generated for free but nothing else here supports it, so shipping it would claim more than is tested.

## What the packages need

Packages cannot run a command at build time from nfpm's config, so the scripts are generated into the tree first and packaged as files.
The output is the same for every architecture, so generate it with the host's binary (`go run ./cmd/botropolis completion bash`) rather than the cross-built one, which an amd64 runner cannot execute for arm64.

| package | bash | zsh | fish |
| --- | --- | --- | --- |
| `.deb`, `.rpm`, Arch (`packaging/nfpm.yaml`) | `/usr/share/bash-completion/completions/botropolis` | `/usr/share/zsh/site-functions/_botropolis` | `/usr/share/fish/vendor_completions.d/botropolis.fish` |
| Homebrew (`packaging/brew/botropolis.rb.in`) | `generate_completions_from_executable(bin/"botropolis", "completion")` puts all three in Homebrew's own completion directories | | |

The AUR packages are built from `~/workspaces/aur/botropolis-git`'s PKGBUILD rather than nfpm, so they need the same three lines there, generated in `package()`.
`tools/check-packages` (`make packages-check`) already asserts the `.deb`, `.rpm` and Arch package carry the same files, so it will catch one format missing them.

## What completes today, and what is worth fixing with it

`botropolis __complete ""` lists every subcommand with its description, and `__complete shell-init ""` offers bash, zsh and fish, from its `ValidArgs`.
**No argument completes beyond that, and two of the gaps are actively wrong rather than missing:**

- `attach`, `stop`, `rm` take a job id and `resume` a session id (`pkg/cli/sessions.go`), and `__complete attach ""` returns no candidates with directive `:0`, which tells the shell to fall back to **file names**. Tab after `botropolis attach` offers files from the current directory, the one place completion would help most.
- `new <dir>` falls back to files too, where only directories make sense.

The first wants a `ValidArgsFunction` on each that reads the snapshot (from the daemon, or directly, as `status` does) and offers job ids with the session's title as the description, with `ShellCompDirectiveNoFileComp`.
The second is `ShellCompDirectiveFilterDirs` on `new`'s first argument.
Neither is needed to ship completion, but shipping completion without them makes the id commands worse than no completion at all, so they belong in the same change.

## How to tell it works

Each shell can be asked what it would complete, without a terminal:

- bash: source the script, then `complete -p botropolis` names the function, and setting `COMP_WORDS`/`COMP_CWORD` and calling it fills `COMPREPLY`.
- zsh: harder to drive non-interactively; `autoload -U compinit && compinit` then checking `$_comps[botropolis]` proves it registered.
- fish: `complete -C 'botropolis shell-'` prints `shell-init`.

That fits beside `testing/integration/packaging/shell_test.go`, which already runs all three shells in CI on Linux and macOS and fails rather than skips a missing one.
The Debian and Fedora install test (`testing/acceptance/packages/install-test.sh`) and the brew test can each assert the installed files are where their shell looks.

## What landed, 2026-10-15 (`e677a69`)

- `make completions` writes `bin/completions/{botropolis.bash,_botropolis,botropolis.fish}` with the host's toolchain, unsetting `GOARCH`; the three `package-*` targets and `tarball` depend on it.
- `packaging/nfpm.yaml` installs them at the paths in the table above; the tarball carries them in `completions/`; the formula calls `generate_completions_from_executable(bin/"botropolis", "completion", shells: [:bash, :zsh, :fish])`.
- `pkg/cli/completion.go`: `attach` and `stop` complete live background job ids, `rm` any background job id, `resume` parked session ids, each titled, all with `ShellCompDirectiveNoFileComp`; `new` completes directories only. `pkg/cli/completion_test.go` drives them through cobra's `__complete`, as each shell's script does.
- `testing/integration/packaging/completion_test.go` asks bash, zsh and fish what `botropolis shell-` completes to through the built binary, on Linux and both macOS runners.

**One premise the item did not have:** cobra's bash script needs the bash-completion package (`_get_comp_words_by_ref`), found by the test's first run.
That is inherent rather than a defect — `/usr/share/bash-completion/completions/` is only read through bash-completion — so the test loads it the way a user's shell does, CI installs it, and the README and the formula's caveats say so.

## Left for the next release (done the same day)

The AUR packages are not built by nfpm and still say 0.1.5, so 0.1.6 never reached them.
At the next AUR update:

- `botropolis` (source): in `package()`, after the binaries,

  ```
  install -d "${pkgdir}/usr/share/bash-completion/completions" "${pkgdir}/usr/share/zsh/site-functions" "${pkgdir}/usr/share/fish/vendor_completions.d"
  ./bin/botropolis completion bash > "${pkgdir}/usr/share/bash-completion/completions/botropolis"
  ./bin/botropolis completion zsh  > "${pkgdir}/usr/share/zsh/site-functions/_botropolis"
  ./bin/botropolis completion fish > "${pkgdir}/usr/share/fish/vendor_completions.d/botropolis.fish"
  ```

- `botropolis-bin`: install the same three from the tarball's `completions/` directory, which exists from the first release after `e677a69`.

The Debian and Fedora install tests assert the files after install, but they run only in a real `packages` publish, so they first run at that release.

## Closed, 2026-10-15

Released as **0.1.7** (`62c50ee`, run 38707120867), whose `packages` publish ran the Debian and Fedora install tests' completion checks for the first time, and passed.
Both AUR packages were stepped through each version, published one at a time: `botropolis` 0.1.6 (`edf14d3`) and 0.1.7 with completion (`e959f16`), `botropolis-bin` 0.1.6 (`f766235`) and 0.1.7 with completion (`aa4d0c6`).
Each was built with `makepkg` before it was pushed, and the 0.1.7 packages were checked to carry all three scripts; the source package's bash script completed `shell-init` through the packaged binary.

**Not tested:** zsh completion is checked by registration, not by driving a completion, which needs a pty script.
