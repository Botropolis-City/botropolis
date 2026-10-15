# Botropolis — roadmap

The plan and its current state.
Every numbered item has its own file in [`roadmap/`](roadmap/); this file is the index and nothing else.

Detail belongs in `roadmap/`, rules and what-exists belong in [DESIGN.md](DESIGN.md), and neither belongs here.
This file grew to 2,689 lines by holding both, which is what it is being kept short to avoid.

## Phases

| phase | what | state |
| --- | --- | --- |
| 7 | UI foundation | shipped 2026-09-18 |
| 8 | City plan | shipped 2026-09-18 |
| 9 | Art pipeline and art pass | shipped 2026-09-18 |
| 10 | Parity with bot-crossing | shipped 2026-09-18 |
| 11 | Info and control | shipped 2026-09-18 |
| 12 | Packaging and polish | shipped 2026-09-18 |
| 13–18 | audit follow-up | done, [`audit-r96.md`](roadmap/audit-r96.md) |
| 19 | Info views | done, [`phase-19-info-views.md`](roadmap/phase-19-info-views.md) |
| 20 | The map has to explain itself | one item open, [`phase-20-explain-itself.md`](roadmap/phase-20-explain-itself.md) |
| 21 | The art batch | done, items 35–68 |

Phases 7–12 were specified in [`phases-7-12.md`](roadmap/phases-7-12.md) and audited at r96.

## Open

| item | what it wants | whose call |
| --- | --- | --- |
| [69](roadmap/069-the-10-percent-cpu-bar-is-missed.md) | the 10% CPU bar: still the bar, or the wrong number for a busy city? | Aria |
| [70](roadmap/070-atlas-start-up-peak.md) | the start-up peak holds two copies of every atlas page; fix is page-at-a-time | nobody, a known fix |
| [66](roadmap/066-115-mb-private-dirty-curiosity.md) | ~115 MB of private dirty unaccounted for, filed as a curiosity | nobody, until it costs something |
| [74](roadmap/074-night-glow-discs-are-back-in-the-demo-frames.md) | night glow draws as a grid of discs in the demo frames, item 37's shape at fit | nobody, a defect to diagnose |
| [75](roadmap/075-a-road-label-is-drawn-under-a-tower.md) | a road's label is drawn under the tower beside it | nobody, a defect |
| [76](roadmap/076-what-the-website-still-needs-filmed.md) | the website's stand-ins: a hero loop, a still per view, a social card, chosen posters, anchors | nobody, filming work |
| [78](roadmap/078-trains-belong-on-the-main-view.md) | the trains run only on the Spend view; put them on the main view as bug 48 did the cars | nobody, Aria asked for it |
| [81](roadmap/081-shell-completion-is-not-packaged.md) | ship `botropolis completion` for bash, zsh and fish in every package; complete session ids and dirs | nobody, Aria asked for it |
| [phase 20 item 3](roadmap/phase-20-explain-itself.md) | the plant's *reading* — the geometry was fixed at r186 and has not been looked at since | Aria, from a frame |

## Waiting on Aria

- **The plant's reading.** Wants a frame, not a measurement.
- **The validation checklist** has its own file, [`validation-checklist.md`](roadmap/validation-checklist.md). **Steps 1 and 8 pass** — step 1 now reconciles by session id rather than by title, which is what item 71 bought. **The other eight need her hands on the keyboard**, and the file says which keys.
- **The plant frames are rendered and waiting** — `docs/screenshots/r320-plant-fountain-aligned.png` and `r320-plant-fountain-behind.png`, the camera turned 180° between them so the fountain moves out from under the flange. If the flange reads as an underside in the first and not the second, the fountain's alignment is the cause; if it reads the same in both, the flange is. Read the premise note on items 23 and 33 first — the flange description was derived from a piece that is no longer the one drawn.
- **Item 69** — whether 10% is still the bar for a busy city, or was always the bar for an idle one.

The nine unadjudicated entries were read one by one on 2026-09-29 and all nine are now closed: item 24 by Aria's ruling that the six undrawn pieces are a deliberate reserve, and items 22, 23, 26, 27, 28, 29, 31 and 33 by adjudication, each with the reason on its own file.
**It was worth doing.** Two facts had been measured five times between them and owned by nothing — the 10% bar being missed, and the atlas start-up peak holding two copies of every page with its fix named and deferred nine phases ago.
Closing those eight without reading them would have retired both silently, which is exactly what the list was there to prevent.

## Background

Written when this roadmap was opened and still the frame for it.

- [why-a-second-pass.md](roadmap/why-a-second-pass.md) — the r64 frame this plan was opened against
- [design-brief.md](roadmap/design-brief.md) — what "modern and intentional" means here
- [art-direction.md](roadmap/art-direction.md) — the kits, and why these ones
- [parity-with-bot-crossing.md](roadmap/parity-with-bot-crossing.md) — what the predecessor had, row by row
- [later.md](roadmap/later.md) — noticed while building, not in a phase; each one a question rather than a plan

## Not doing

Planets, orbit mode, a 3D renderer, network serving, quality presets beyond render scale and reduced motion, animated faces.
Each is either bot-crossing's setting rather than a feature, or a cost the footprint principle rules out.

## Where things are

| | |
| --- | --- |
| `roadmap/NNN-*.md` | one file per numbered item, 1–68 |
| `roadmap/phase-*.md` | the phases that carry their own sub-items |
| `roadmap/assets/` | references and crops belonging to an item |
| `docs/screenshots/` | frames, cited by revision |
| `docs/references/` | annotated crops from investigations |
| [DESIGN.md](DESIGN.md) | what exists, and the rules the work is held to |

Items are numbered in one series, 1–68.
Section 8's bugs 1–34 and the art batch's 35–68 continue the same numbering; phases 19 and 20 have their own small series, cited as "phase 20 item 3".
There are two bug 18s — road tiles, and rovers parking — both fixed, both left renumbered as written, because a dated record that gets tidied stops being a record.
Nothing cites either by number.

## Status, 2026-09-29

72 items, 68 closed, 4 open, 0 unadjudicated — counted by number.
By *file* it is 69 closed of 73, because the two bug 18s are one number and two entries.
Both figures are the same fact; this line quotes the first.
Gate green: `go build`, `go vet`, 22 test packages, `make lint` 0 issues.
The inventory is [`inventory-2026-09-29.md`](roadmap/inventory-2026-09-29.md).
