# Botropolis — design and first-pass plan

Botropolis is a single pane of glass for every Claude Code session on this machine,
drawn as a city.
Each project is a district, each session is a building with a worker in it,
and every object on the map stands for exactly one thing you can read by hovering it.

It grew out of [bot-crossing](https://github.com/jarrenrocks/bot-crossing),
whose two best ideas it keeps: a per-harness adapter seam,
and a sticky layout so the map you have learned does not move under you.
Everything else is redesigned around how the sessions on this machine are actually used
(see [docs/usage-profile.md](docs/usage-profile.md)).

## Principles

1. **Every object means one datum.**
   Nothing is drawn unless hovering it shows the number it stands for.
   No ambient crowds, no decorative buildings, no size-by-transcript-bytes.
2. **CLI-first.**
   The desktop app's focus bookkeeping is never relied on.
   "Needs you" is derived from the process and the transcript, or pushed by a hook.
3. **Sessions live in Claude's daemon; terminals are views.**
   Sessions are started with `claude --bg` and opened with `claude attach`.
   Closing a terminal never ends a session.
4. **Only the `claude` CLI is a write path.**
   `~/.claude` is read-only to us.
   Start, stop, resume, and delete go through `claude` subcommands, never through its files or its private daemon socket.
5. **Small footprint.**
   The daemon is a few megabytes and idles on inotify and hook pushes.
   The renderer is a separate process you can close.
6. **Only the states that want something from you animate for attention.**

## What exists

| | |
| --- | --- |
| [architecture.md](design/architecture.md) | packages, processes and what talks to what |
| [data.md](design/data.md) | what the data can tell us |
| [session-lifecycle.md](design/session-lifecycle.md) | how a session is started, watched and ended |
| [city-and-rendering.md](design/city-and-rendering.md) | the city, the projection, the view and the plan |
| [chrome.md](design/chrome.md) | strip, footer, cards, panels |
| [info-views.md](design/info-views.md) | the nine views, their legends and what each says |
| [sprite-pipeline.md](design/sprite-pipeline.md) | Blender, the kits, the packer, the atlases |
| [milestones.md](design/milestones.md) | what shipped, in order |
| [parity-and-ship.md](design/parity-and-ship.md) | bot-crossing parity, what went beyond it, what shipping means |
| [what-it-costs.md](design/what-it-costs.md) | measured cost — CPU, memory, binary, package |
| [open-questions.md](design/open-questions.md) | what is not decided |

## Rules

Each of these was paid for by a specific mistake, and each is stated so it can be applied rather than admired.

| | |
| --- | --- |
| [a-number-can-be-right-and-mean-nothing.md](rules/a-number-can-be-right-and-mean-nothing.md) | a correct measurement of the wrong thing |
| [a-plausible-wrong-number.md](rules/a-plausible-wrong-number.md) | why the reasonable-looking figure is the dangerous one, and the three questions to ask |
| [a-second-independent-route.md](rules/a-second-independent-route.md) | what verification actually requires, and why six honest greens meant nothing |
| [a-green-test-is-not-a-guard.md](rules/a-green-test-is-not-a-guard.md) | mutate it red before trusting it |
| [a-document-edited-in-turns.md](rules/a-document-edited-in-turns.md) | a file can contradict itself with every edit correct |
| [two-things-that-must-agree.md](rules/two-things-that-must-agree.md) | every pair that must agree, and what makes them |
| [why-the-category-colours-are-legal.md](rules/why-the-category-colours-are-legal.md) | the palette constraint, and a map's real cap of three |

## Where the rest is

The plan and its open items are in the [Botropolis-City meta repository](https://github.com/Botropolis-City/meta): [ROADMAP.md](https://github.com/Botropolis-City/meta/blob/main/ROADMAP.md), with one file per numbered item in [`roadmap/`](https://github.com/Botropolis-City/meta/tree/main/roadmap).
Working under the meta repository's root, that is `../ROADMAP.md`.
Frames are in `docs/screenshots/`, cited by revision.

This file was 877 lines before the split, holding the record, the rules and the cost tables together.
It is kept to an index so that adding a rule does not mean growing the thing everyone reads first.
