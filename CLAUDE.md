# Botropolis

Go + Ebitengine isometric city that visualises Claude Code sessions.
Districts are projects, buildings are sessions, the power plant is tokens, radio towers are MCP.

## Where writing goes

This is the rule that was never written down, and the roadmap grew to 2,689 lines because of it.

| file | holds | does not hold |
| --- | --- | --- |
| `ROADMAP.md` | the index: phases, what is open, what is waiting on Aria | any item's detail |
| `roadmap/NNN-*.md` | one numbered item each — the report, the diagnosis, the fix, the frame | rules that outlive the item |
| `roadmap/assets/` | crops and references belonging to one item | frames, which go in `docs/screenshots/` |
| `DESIGN.md` | the index, plus the principles | sections of prose |
| `design/*.md` | what exists — architecture, the city, the pipeline, measured cost | lessons |
| `rules/*.md` | one rule each, paid for by a named mistake | anything not yet generalised |

**A file that is an index stays an index.**
If detail is being added to `ROADMAP.md` or `DESIGN.md`, it belongs in a file underneath them instead.
Both were kept short deliberately so that adding an item or a rule does not grow the thing everyone reads first.

**Items are numbered in one series, 1–68 and counting.**
Numbers are never reused and never renumbered — `roadmap/` filenames, commit messages and code comments all cite them.
There are two bug 18s, left as written, because a dated record that gets tidied stops being a record.

**Item status lives in the item's own file**, on the line under its title: `Done`, `Fixed`, `Open`, `Unadjudicated`, with the date and revision.
`ROADMAP.md`'s open table is derived from those lines, so it is checked rather than remembered — a hand-maintained summary of machine-checkable state always drifts, which it did.

**The skills `roadmap-add`, `roadmap-close` and `roadmap-status`** (in `.claude/skills/`) carry out this contract step by step: search before numbering, number from `git ls-files`, status on line 3, one row in the open table, `make docs-check`, then commit to `main`.

**`make docs-check` enforces all of this**, and runs as part of `make lint`.
It verifies that every relative link resolves *from its own file's directory*, that no `§N` citation to the pre-split roadmap survives, that every item declares a status, and that `ROADMAP.md`'s open table lists exactly the items whose status is `Open`.
It also refuses an empty corpus — the four indexes must be non-empty and every item number from 1 to the highest must have a file — because the other checks all pass on nothing.

Every check was mutated red before the script was trusted, and **five of its own versions could not fail**:
the first link check tried the bare path before the relative one, so every root-level target resolved from the repo root;
the first corpus check passed on a directory containing four empty files and no `roadmap/`;
and the version after that took its ceiling from the working tree, so deleting the two highest-numbered items lowered `max()` and it reported "1-66, none missing".
The ceiling now comes from `git ls-files`, which a working-tree deletion cannot move — but its `except: pass` silently restored the fault whenever git could not be reached, and the summary then read exactly like a healthy run.
Running outside a checkout is legitimate, so the fallback stays; **it just is not silent any more**, and the green says which source the bound came from.
And every check up to that point ran in one direction — each asked whether a link points at a file that exists, and none asked whether a file is pointed *at*, so a document could sit in the tree unreachable by navigation with everything green.
That was true of three files the split itself created: `design-brief.md`, `later.md` and `why-a-second-pass.md` were in the tree and linked from nowhere.
Item files are exempt, because the numbering check is their reachability guarantee; everything else has to be linked from somewhere.

The pattern across all five is one thing: **a guard fails where its own inputs come from.**
A fallback is a way for a checker to pass, an empty corpus satisfies every check that iterates over it, a bound derived from the thing it measures can be lowered by the fault it exists to catch, and a check that only follows links outward cannot see what nothing points to — which is item 59's shape, a piece nothing draws being a piece nothing validates.
Mutating a guard tests the guard; mutating what the guard measures itself *against* tests whether it has a fixed reference at all.

**A different sentence, and the one this repo has actually been bitten by: a guard that is not invoked cannot fail.**
`make lint :: docs-check` is one line in a Makefile with nothing guarding it — delete it and all six checks go quiet at once behind a green gate.
Asserting that dependency from inside the thing it invokes is circular, and grepping the Makefile is a string match on a build file, so the answer is a **second invocation path**: `testing/acceptance/docs_test.go` runs `tools/check-docs` too, so the checks survive losing either caller.
Verified by removing the Makefile dependency and orphaning a file at the same time — `make lint` went green and the test failed.
A comment would not have been enough, and this repo is the proof: `tools/shrink-pngs` sat in the Makefile for fourteen revisions while the path that actually ran bypassed it, at 6.6 MB of binary, with the Makefile entry right there the whole time.

The same sentence caught that second caller too: **a cached test is a test that did not run.**
Go keys the test cache on files the test binary itself opens, and `check-docs` reads the documentation in a subprocess, which the cache cannot see — so with an orphaned file present the test reported `ok (cached)` while the script failed on the same tree, and `make test-acceptance` passes no `-count=1`.
`docs_test.go` now opens every file the script inspects, which puts them in the key: identical runs still cache, and any documentation change re-runs it.

**And `DESIGN.md`'s index is load-bearing as a guard, not only as navigation.**
Deleting `design/` or `rules/` is caught because the index links into them.
Tidying an index down to prose would remove a check without appearing to.
## Releasing

**The version is one line in `VERSION`, and everything reads it** — the Makefile's ldflags, `packaging/nfpm.yaml`, and the release workflow.
Before this there were three copies of the ldflags string carrying three different values, which is how the sibling project this pattern came from ended up with a package twelve patch versions behind its source.
`make version-check` fails, naming files, when `VERSION`, `CHANGELOG.md` and the built binaries disagree, and it runs as part of `make lint`.

**CI builds what the release publishes.** Every push to `main` produces the tarball, `.deb`, `.rpm`, Arch package and `SHA256SUMS` as a run artifact.
The release workflow is `workflow_dispatch` only and rebuilds nothing: it reads `VERSION`, refuses a version already tagged, downloads that commit's artifact, takes its notes from the changelog section for that version, and creates the tag with `--target` so the tag points at the commit that was tested.
It takes a `dry_run` input that validates all of that and creates nothing.

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

Two sessions, at Aria's request.
One owns `ROADMAP.md`, `roadmap/`, numbered items, audits and relaying to Aria; the other writes code, renders frames to `docs/screenshots/` and packages to `~/workspaces/aur/botropolis-git`.
Aria judges the frames.
Relaying to Aria is the roadmap session's half — check before sending, or she gets the same thing twice.

**Cross-checking is capped at two rounds (Aria, 2026-09-29).** Her ruling: *"Keep it, cap the rounds"* — verify each other's work once, and take a second round only if the first found something material.
Otherwise write the finding in the file and skip the message.
It was capped after a day of seven rounds on one 90-line script in which **every round found a real defect** and the day still shipped no features.
So the cap is not a judgement that the cross-checking was wrong; it is a budget. The value is real and bounded, and the rounds after the second are where it stops paying.

## Gotchas that have each cost a session

- **`log.showSignature=true` is set repo-locally**, so `git log --oneline | wc -l` counts double and `git log --format=%s` returns the signature ahead of the subject.
  Pass `--no-show-signature` in anything scripted; `git rev-list --count` and `git rev-parse` are unaffected, which is why `pkgver()` has never broken.
- **Repo-local identity and signing**: `Aria Vesta <dev@ariavesta.com>`, SSH signing with `~/.ssh/id_ariavesta_sign_ed25519`.
  Any new repo for this project needs the same four `git config` lines before its first commit, or the global `commit.gpgsign` signs with the work key.
- **The atlas is embedded from `pkg/assets/kits`**, not `pkg/render`, which is the package that draws it.
- **Anything counting sprite usage must exclude `_test.go` and must match string literals rather than file text.**
  A corpus containing the test always agrees with the test, and a doc comment naming a piece is not a caller. See `rules/a-green-test-is-not-a-guard.md`.
- **Restarting the daemon does not restart the map.** When a frame disagrees with the screen, check for a client process first.
