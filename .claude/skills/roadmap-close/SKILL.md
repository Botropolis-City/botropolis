---
name: roadmap-close
description: >-
  Close a numbered botropolis roadmap item as Done or Fixed once its work has
  landed, or adjudicate it closed with a reason. Use after finishing an item's
  work, or when asked to close, mark done or resolve a roadmap item. Updates
  the item's status line with the date, commit and revision, removes every
  link to it from ROADMAP.md, runs make docs-check, and commits to main.
---

# Close a roadmap item

## 1. Confirm it is done

Read the item and check what it asked for against what landed: the commit, the tests, the frame.
If only part landed, do not close it. Add what landed to the body and leave it **Open**.
If it is closed by a decision rather than a fix, the reason goes in the body. "Not doing", with why, is a valid close.

## 2. Rewrite the status line

Line 3 of `roadmap/NNN-*.md`. Keep the filing history:

```markdown
**Done 2026-10-11 (`abc1234`, r380). Filed 2026-10-10 (r376) at Aria's request.**
```

- `Done` for work and requests, `Fixed` for defects. The checker treats anything not starting with "Open" as closed.
- The commit is the one that did the work, not this one; its revision is `git rev-list --count <sha>`.
- Add a short closing section to the body: what was done, the frame or test that shows it, and any deviation from what the item asked, flagged for Aria.

## 3. Unlist it everywhere

`make docs-check` treats **any** link to the item in `ROADMAP.md` as listing it as open, so remove all of them:

```sh
grep -n 'roadmap/NNN-' ROADMAP.md
```

That catches the open table row and any `## Waiting on Aria` bullet.

## 4. Check, commit, push

```sh
make docs-check
git add ROADMAP.md roadmap/NNN-*.md
git commit               # "Close item NNN: <title>"
git pull --rebase github main && git push github main
```

## Report

Name the item, what closed it, and anything left for Aria to look at, such as a frame to judge.
