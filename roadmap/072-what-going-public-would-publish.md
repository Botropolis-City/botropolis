# 72. Going public would publish the employer's internal repository structure, and history rewriting is the only way back

**Done by 2026-10-10: Aria made the repository public, now at `github.com/Botropolis-City/botropolis` (item 77). Filed 2026-09-29; the rewrite it called for landed 2026-09-30.** She chose to rewrite this repository rather than publish a fresh one, and `main` was rewritten and force-pushed on 2026-09-30: `tools/make-fixtures.py` extended to rename work context, `docs/usage-profile.md` reduced to aggregates, all 225 screenshot blobs dropped, and every employer token replaced across blobs and commit messages. **What the rewrite missed: `refs/tags/v0.1.0`.** It pointed at `af0f775b`, not an ancestor of the rewritten `main`, and that commit still carried the full dump — so the tag and its published release re-exposed everything `main` had just been scrubbed of. Found on 2026-09-30 while surveying the repository for release work, not by the rewrite's own verification, which swept what was reachable from `main` and never asked what else the remote advertised. Release and tag deleted; `git ls-remote` now returns only `refs/heads/main`, and every ref it returns is an ancestor of it.

The repo is private today and phase 12's publish step is deliberately blocked on that.
Flipping it is one click and is not reversible in the way it sounds: once public, the clone and the history are out.

## How it got here

`tools/analyze-history.py` scans `~/.claude/projects`, which is **every directory Aria has ever run Claude in** — all of her the employer work included — and prints per-project and per-session tables.
Its output was committed as `docs/usage-profile.md` in this repo's **first commit**, `633bdcf` (2026-09-16, *"Design and first-pass plan for Botropolis"*).

It was not a stray paste. The file's fourth line says *"It is what the design in DESIGN.md is shaped around"*, and that is true: the design needed the aggregates — CLI-only, bimodal session length, concurrency, context as the scarce resource — and those are the headline bullets at the top.
**The per-project and per-session tables came along with them**, because the tool prints everything and the whole output was kept as the evidence.

The rest followed from working on the real machine: the Go tests took the paths that were in front of whoever wrote them, and the frames are renders of Aria's actual city.
`sessions.json`, which the same tool writes, was **never committed** — checked across all branches.

## What is in the tree

**1. `docs/usage-profile.md` — the dump, in three regions.**

*Lines 53–73, "Per project":* twenty rows naming the employer's GitHub organisation and seven of its repositories, plus three Bitbucket paths, each with session count, active hours, output tokens, cache reads, **PR counts** and recency. An inventory of their repositories with effort and throughput against each.

*Line 115, "Skills":* three plugin namespaces belonging to the employer, with usage counts in the hundreds — internal tooling names.

*Lines 129–144, "Top 15 sessions by active time":* session titles, which are ticket identifiers — `'Ticket 604'`, `'Ticket 602'`, `'Issue 597'`, `'Issue 613'`, `'PROJ-1002'`, `'PROJ-1001 Medi…'`, `'Pull request 468…'` — each with hours, prompt counts and tokens. **`PROJ-1001` beside `bitbucket/tidalUpland/driftingNettle` maps an issue-tracker key to its repository.**

**2. Go sources — 71 occurrences of the organisation and two repository names across 29 files, and 292 of a third.**
These were not neutral fixtures: they hardcoded real absolute paths including the employer's organisation and repository, and a real PR number against one of them.
`pkg/commands/status.go:50` — production, not a test — gained a comment naming two of that repository's sessions **on 2026-09-29**, out of item 71's investigation, so the tree was still accumulating them.

**3. `docs/screenshots/` — verified by looking, not assumed.**
A frame checked by opening it showed district plates reading the employer's repository names, and the strip across the top showed a real daily spend figure beside the needs-you, PR and error counts.
Roughly 45 of the 84 frames are full-window (1100x760, 2200x1520, 1920x1120) and so carry the strip; the rest are crops. Item 40 made the district plate permanent by design, so this is what the program is supposed to draw.

**4. All 323 commits**, since it has been there since the first one. Nothing worse is hiding in deleted history — the deleted paths are Kenney assets and superseded code.

## The lesson, which is the one this project keeps relearning

A rewrite verified by sweeping `main` proves something about `main`.
The question that mattered was not *is the history clean* but *what does the remote hand to someone who asks for everything*, and those differ by exactly the refs nobody thought to enumerate.
`git ls-remote` answers the second one in a line, and the check now in use is to walk every ref it returns and assert each is an ancestor of `main` — a bound taken from an independent source rather than from the thing being checked, which is [a-second-independent-route.md](../rules/a-second-independent-route.md) again.

Worth noting what it cost to find: nothing, because the repository was still private.
The same miss on a public repository is not recoverable.

## Why this collides with the item 58 ruling

Aria ruled item 58 **leave it** — no history rewrite.
Item 58's own text says the trigger to revisit is *the repo going public*, and that going public is also **the last moment a rewrite is cheap, before there are clones to invalidate**.

So the two answers meet here: **scrubbing the working tree does not unpublish anything, because all 322 commits go with the repo.** Removing the dump from history means a rewrite, which is exactly the thing she ruled against, at exactly the moment its own filing named as the one to reconsider.

## The options, and none of them is free

1. **Rewrite history now, then publish.** The only option that actually removes it. Cheapest it will ever be — one clone, hers. Invalidates every commit hash, including the ones cited throughout `roadmap/`, `design/` and `rules/`, which is a real cost to a project whose records are half its value.
2. **Publish a fresh repo with no history.** A single squashed initial commit from the current tree, scrubbed. Keeps the private repo intact as the record, publishes a clean one. Loses the commit-level history publicly, which for this project is a genuine loss — the reasoning is in the messages.
3. **Scrub the tree, publish with history, accept what is in it.** Fastest, and the dump stays reachable forever in the history of a public repo. Not recommended, and the reason to state it is that it is what happens by default if nobody decides.
4. **Stay private, publish only the package.** The AUR is blocked on upstream being private, so this means not publishing at all.

**Recommendation: (2).** The history's value is to the two of us and to Aria, not to a reader, and it stays available privately. A squashed public repo costs one afternoon, needs no rewrite of the private record, and the commit hashes cited across `roadmap/` keep resolving in the repo where they were written.

**Whatever she picks, `docs/usage-profile.md` has to be rewritten to aggregates before anything is published** — the headline findings at the top of that file are the part the design was actually shaped around, and they are already aggregate. It is only the per-repo table that has to go.

## Closed

Aria made the repository public, and said so on 2026-10-10; it was public before this session knew.
Checked against the public remote that day:

- Every ref `git ls-remote` advertises (`main` and the tags `v0.1.0` to `v0.1.4`) is an ancestor of `main`.
- Every image in the tree is on `tools/check-publishable`'s reviewed list.
- **The token check was not run**: `.publishable-tokens` is not on the machine that checked.
  Aria ruled the same day that the list stays only where it already is, because copying the employer's names anywhere else, CI included, is itself collecting the employer's data.
  Both guards now skip the token check when the list is absent and say so in their output rather than passing quietly (`a254cbd`, r384).
  So the history scan this item rests on has to be run on the machine that holds the list; nothing here says it has been since the 2026-09-30 rewrite.

