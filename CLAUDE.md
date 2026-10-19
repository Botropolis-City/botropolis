# Botropolis

Go + Ebitengine isometric city that visualises Claude Code sessions.
Districts are projects, buildings are sessions, the power plant is tokens, radio towers are MCP.

## Where writing goes

| file | holds | does not hold |
| --- | --- | --- |
| `DESIGN.md` | the index, plus the principles | sections of prose |
| `design/*.md` | what exists — architecture, the city, the pipeline, measured cost | lessons |
| `rules/*.md` | one rule each, paid for by a named mistake | anything not yet generalised |
| `docs/screenshots/` | frames, cited by revision | item crops, which live with the roadmap |

**The roadmap is not in this repository.**
`ROADMAP.md`, the numbered items in `roadmap/` and their crops moved to the Botropolis-City meta repository on 2026-10-15, at r410, and their history before that is still here (`git log -- ROADMAP.md roadmap/`).
Working under the meta repository's root, it is one directory up: `../ROADMAP.md`, `../roadmap/`.
Its rules — item numbering, status lines, the open table, the roadmap skills — and the two-session working arrangement are in `../CLAUDE.md`, which Claude Code loads from here too, because it reads every `CLAUDE.md` from the working directory up.
Revisions cited as `r<N>` there count this repository's commits.

**A file that is an index stays an index.**
If detail is being added to `DESIGN.md`, it belongs in a file underneath it instead, so that adding a rule does not grow the thing everyone reads first.

**`make docs-check` enforces what is left here**, and runs as part of `make lint`.
It verifies that every relative link resolves *from its own file's directory*, that no `§N` citation to the pre-split roadmap survives, that the indexes are non-empty, and that nothing in `design/` or `rules/` is linked from nowhere.
The roadmap's checks moved with the roadmap, and so did the history of how five versions of this script could not fail; it is in `../CLAUDE.md`, and its moral holds here: **a guard fails where its own inputs come from.**

**A guard that is not invoked cannot fail.**
`make lint :: docs-check` is one line in a Makefile with nothing guarding it — delete it and every check goes quiet at once behind a green gate.
Asserting that dependency from inside the thing it invokes is circular, and grepping the Makefile is a string match on a build file, so the answer is a **second invocation path**: `testing/acceptance/docs_test.go` runs `tools/check-docs` too, so the checks survive losing either caller.
Verified by removing the Makefile dependency and orphaning a file at the same time — `make lint` went green and the test failed.
A comment would not have been enough, and this repo is the proof: `tools/shrink-pngs` sat in the Makefile for fourteen revisions while the path that actually ran bypassed it, at 6.6 MB of binary, with the Makefile entry right there the whole time.

The same sentence caught that second caller too: **a cached test is a test that did not run.**
Go keys the test cache on files the test binary itself opens, and `check-docs` reads the documentation in a subprocess, which the cache cannot see — so with an orphaned file present the test reported `ok (cached)` while the script failed on the same tree, and `make test-acceptance` passes no `-count=1`.
`docs_test.go` now opens every file the script inspects, which puts them in the key: identical runs still cache, and any documentation change re-runs it.

**And `DESIGN.md`'s index is load-bearing as a guard, not only as navigation.**
Deleting `design/` or `rules/` is caught because the index links into them.
Tidying an index down to prose would remove a check without appearing to.

## Art

**No picture, texture or mesh comes from an image model** — only from a human artist, an openly licensed pack, or geometry built by hand in code (Aria, 2026-10-18, item 123); read [`design/art-sources.md`](design/art-sources.md) before touching any image, and `make lint` runs `tools/check-asset-sources`, which fails on any image under `pkg/assets/` without a traceable source.

## Releasing

**The version is one line in `VERSION`, and everything reads it** — the Makefile's ldflags, `packaging/nfpm.yaml`, and the release workflow.
Before this there were three copies of the ldflags string carrying three different values, which is how the sibling project this pattern came from ended up with a package twelve patch versions behind its source.
`make version-check` fails, naming files, when `VERSION`, `CHANGELOG.md` and the built binaries disagree, and it runs as part of `make lint`.

**CI builds what the release publishes.** Every push to `main` produces the tarball, `.deb`, `.rpm`, Arch package and `SHA256SUMS` as a run artifact.
The release workflow is `workflow_dispatch` only and rebuilds nothing: it reads `VERSION`, refuses a version already tagged, downloads that commit's artifact, takes its notes from the changelog section for that version, and creates the tag with `--target` so the tag points at the commit that was tested.
It takes a `dry_run` input that validates all of that and creates nothing.

**The release runs the benchmarks, not every push** (Aria, 2026-10-18, item 104): it runs `make bench` on the same runner kind each time, attaches `bench.txt` to the release, and writes a benchstat comparison with the previous release's to the run summary.
It reports and never gates; the acceptance tests gate on steps, ticks and work.
Per push, on shared runners, one run against the last said little and gated nothing, so it moved here, where release to release is the comparison worth reading.

**The release also publishes every package under a fixed name** (`tools/release-aliases`), and botropolis.city's install commands link `/releases/latest/download/<fixed name>`; a final step downloads each through that link and fails if it is not this release's file. That is why a release needs no website edit, and why those names must never change.

**The release then publishes the APT and RPM repositories at pkg.botropolis.city** (`.github/workflows/packages.yml`, also runnable by hand to republish).
It rebuilds both from the last three releases with `tools/build-package-repos`, installs from the result in clean Debian and Fedora containers before uploading, and installs from the live host after.
The signing key's master copy is the 1Password item "Botropolis Packages Signing Key"; CI has it as `PKG_SIGNING_KEY`, and the builder refuses any key that is not `packaging/repo/botropolis.asc`. It expires 2029-10-10: extend it with `gpg --quick-set-expire` and republish the public key before then.
`make package-repos package-repos-test` does the same locally.

**The same workflow publishes the Homebrew tap to `pkg.botropolis.city/brew`** (`tools/build-brew-tap`), as a bare git repository read over git's static HTTP protocol, which is all a bucket can serve.
Its formula builds from the release's source archive, so no Mac downloads an unsigned binary; it is install-tested in a Linux Homebrew container and on a macOS runner before upload, and from the live host after.
Each publish continues the published tap's history, because `brew update` needs a fast-forward, and the builder refuses rather than start a new history when it cannot clone the live one.
**The workflow has no way to start a new history**, by design: it was started once, on 2026-10-15, and a second start would leave every Mac that has tapped it on a history the published one no longer continues. `tools/build-brew-tap --new` exists for local test builds only.
`make brew-tap brew-tap-test` does the same locally.

To cut one: edit `VERSION`, add the matching `CHANGELOG.md` section, `make version-check`, push to `main`, let CI finish, run the workflow.

**A workflow that does not parse is not a workflow that fails.**
GitHub lists it by path rather than by name and fails it before any step runs.
A single-line `run:` whose value contains `": "` is the way in, and `inputs.some-name` is another — a hyphen parses as subtraction in a GitHub expression.
`make lint` runs `tools/check-workflows` so both fail locally.

**The repository is public, at `github.com/Botropolis-City/botropolis`, so every push publishes. Run `make publishable` before pushing anything that adds an image or touches history.**
The module path is `botropolis.city/botropolis`, served by the website's `go-import` tag, so it does not change if the repository moves again.
`make publishable` asserts three things, and each exists because the scrub missed something of that shape.
No employer token in any blob or commit message, anywhere in history — checked only on a machine that has `.publishable-tokens`, the list of the employer's names, which is never copied anywhere else, CI included, because holding it elsewhere would be collecting the employer's data; without it the check is skipped and the output says so.
Every ref `git ls-remote` advertises is an ancestor of `main` — the first rewrite scrubbed `main` and left `refs/tags/v0.1.0` pointing at unscrubbed history, because the verification swept what was reachable from `main` and a tag is not.
And every image in the tree is on a reviewed list — `docs/botropolis.gif` showed a project name, MCP vendor names and a real daily spend for nine days after the scrub, because the scrub filtered `docs/screenshots` and the GIF is not in it.

**An image not on that list fails the check, and the way to pass is to look at it and add it.**
Three misses had one shape: the scrub named a path and the exposure was at a sibling path.
A check that enumerates what exists and demands each item be accounted for cannot miss that way; a check that enumerates what you remember always can.

## Working arrangement

Two sessions, at Aria's request: the roadmap session at the meta repository's root, and the code session here.
The arrangement and the cap on cross-checking are in `../CLAUDE.md`.

## Gotchas that have each cost a session

- **`log.showSignature=true` is set repo-locally**, so `git log --oneline | wc -l` counts double and `git log --format=%s` returns the signature ahead of the subject.
  Pass `--no-show-signature` in anything scripted; `git rev-list --count` and `git rev-parse` are unaffected, which is why `pkgver()` has never broken.
- **Identity and signing depend on the machine**: every commit is `Aria Vesta <dev@ariavesta.com>`, signed with Aria's own key.
  On her own machine that is the global git config, GPG key `C39F068679829540`, with nothing set per repository.
  On a machine whose global identity is someone else's, set the repo-local identity and signing before the first commit (there it was SSH, `~/.ssh/id_ariavesta_sign_ed25519`), or the global `commit.gpgsign` signs with that machine's key.
  Either way, `git verify-commit HEAD` names `dev@ariavesta.com`.
- **The atlas is embedded from `pkg/assets/kits`**, not `pkg/render`, which is the package that draws it.
- **Anything counting sprite usage must exclude `_test.go` and must match string literals rather than file text.**
  A corpus containing the test always agrees with the test, and a doc comment naming a piece is not a caller. See `rules/a-green-test-is-not-a-guard.md`.
- **Restarting the daemon does not restart the map.** When a frame disagrees with the screen, check for a client process first.
