---
name: roadmap-status
description: >-
  Summarise the botropolis roadmap from the item files themselves: what is
  open, whose call each item is, and what is waiting on Aria. Use when asked
  what's on the roadmap, what's open, what to work on next, or what needs
  Aria's decision. Read-only.
---

# Roadmap status

The item files are the source of truth; `ROADMAP.md`'s open table is derived from them, and `make docs-check` proves the two agree.
Run the check first, so a summary is never built on an index that has drifted:

```sh
make docs-check
```

Then read each open item's status line and title:

```sh
for f in $(git ls-files 'roadmap/[0-9][0-9][0-9]-*.md'); do
  s=$(sed -n 3p "$f")
  case "$s" in "**Open"*) printf '%s\n  %s\n  %s\n' "$f" "$(head -1 "$f")" "$s" ;; esac
done
```

Report in three groups:
1. **Waiting on Aria**: items whose status names her call, with the decision each needs, in one line.
2. **Ready to do**: "nobody's call" items, with what the fix touches if the item says so.
3. **Parked**: open items filed as curiosities or deferred until something costs.

Note `phase-20-explain-itself.md`'s open sub-item separately; it is not numbered, and the check does not see it.
Do not edit anything. If the check fails, report the failure first; it is the more important finding.
