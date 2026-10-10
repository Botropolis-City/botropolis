---
name: roadmap-add
description: >-
  File new work on the botropolis roadmap as numbered items. Use when asked to
  add something to the roadmap, file a bug or request, or record a defect or
  ask for later. Researches the item against the code first, checks it is not
  already filed, writes roadmap/NNN-*.md with its status line, lists it in
  ROADMAP.md's open table, runs make docs-check, and commits to main.
---

# Add a roadmap item

CLAUDE.md's "Where writing goes" is the contract; this fills it in.
An item is one numbered file in `roadmap/`; `ROADMAP.md` is an index and only gets one row per item.

## 1. Check it is not already there

Search before numbering, because a duplicate is worse than a late item:

```sh
grep -rliE '<two or three distinctive words>' roadmap/ ROADMAP.md
```

Read anything that matches.
If an existing item covers it and is **Open**, add to that item instead of filing a new one.
If it covers it and is **Done** or **Fixed** and the problem is back, file a new item that cites the old one by number: numbers are never reused, and a closed item's record is not reopened.

## 2. Research it

An item has to let the next session start without re-deriving anything.
Find the mechanism in the code — the file, the function, the line that gates it — and say what the fix would touch, including the tests that pin the current behaviour and the comments it would make wrong.
Cite evidence a reader can reproduce: a frame in `docs/screenshots/`, a shot in `dist/demo/` with pixel coordinates, a command and its output.
Do not add images under `roadmap/assets/` without putting each on the reviewed list in `tools/check-publishable`; citing a reproducible shot is usually enough.
If the request rests on a belief that turns out to be wrong, say so in the item and in the reply, with what is actually true.

## 3. Number it

```sh
git fetch -q github && git status -sb | head -1     # pull first if main is behind
git ls-files roadmap/ | grep -oE '^roadmap/[0-9]{3}' | sort -u | tail -1
```

The next number is one past the highest tracked, zero-padded to three.
The revision the item cites is the commit that files it: `$(( $(git rev-list --count HEAD) + 1 ))`, written `r<N>`.

## 4. Write it

`roadmap/NNN-<kebab-case-of-the-title>.md`, with the title on line 1, a blank line 2, and the status on line 3. `make docs-check` reads line 3 and nothing else:

```markdown
# 80. <Title as a statement of the problem or the want>

**Open. Filed YYYY-MM-DD (rN), <where it came from>. <Whose call — "Nobody's call — a defect" or "Aria's call — <what she decides>">.**

<Body.>
```

- When Aria asked for it, say so on the status line and quote her words in the body.
- Newline after every sentence, as the repo's markdown convention requires.
- Say whose call it is honestly. "Nobody's" means the fix is known and only needs doing.

## 5. Index it

Add one row to the `## Open` table in `ROADMAP.md`, above the `phase 20 item 3` row:

```markdown
| [80](roadmap/080-the-slug.md) | <what it wants, one line> | <whose call> |
```

If it waits on Aria, add a bullet under `## Waiting on Aria` naming the decision.
Detail stays in the item; the row is one line.

## 6. Check, commit, push

```sh
make docs-check          # must report the new count, none missing, open table matches
git add ROADMAP.md roadmap/NNN-*.md
git commit               # "File item NNN: <title>", with the why in the body
git pull --rebase github main && git push github main
```

The roadmap session may commit at the same time; rebase rather than merge, and re-run `make docs-check` if the rebase touched `ROADMAP.md`.
Work is trunk-based: commit to `main`, no branch or PR.

## Report

Say what was filed, by number, and what the research changed: an existing item it extends, a premise that was wrong, a decision it needs from Aria.
