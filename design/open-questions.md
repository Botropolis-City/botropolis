## Open questions

All settled; the decisions are in the meta repository's [design-open-questions.md](https://github.com/Botropolis-City/meta/blob/main/roadmap/design-open-questions.md) and [art-direction.md](https://github.com/Botropolis-City/meta/blob/main/roadmap/art-direction.md).
The history of each is kept here because it explains code that still exists.

- **Foreground-started sessions.**
  Settled 2026-09-18: no "convert to background" affordance.
  The shell helper is sourced, so new sessions start in the background,
  and there is no CLI to background a running foreground session anyway (the CLI's own dialog is mid-turn only).
  Parked-when-you-quit, resumed-in-the-background-when-you-click is the behaviour.
- **Cost and the context window.**
  Settled 2026-09-17: the CLI writes `cost-state` records into the transcript with `totalCostUSD`
  and a per-model token and cost breakdown,
  so the read model takes the last one as-is and never prices tokens itself.
  The context window is inferred, not looked up: a `[1m]` model id or cost-state key means 1M;
  a context that was ever larger than 200k proves 1M (context cannot exceed the window);
  otherwise 200k is trusted only for the opus/sonnet/haiku families and anything else shows no percentage rather than a guess
  (a session that switched to `claude-fable-5-1` mid-way was reading 463% before this rule).
  One API message is written as several `assistant` records (one per content block, `apiBlockIndex`)
  that repeat the same `usage`, so token totals must be deduplicated by `message.id`.
- **`teams/`, `tasks/` and `plans/`.**
  `teams/` is read for roads (member cwds route `SendMessage` traffic between districts); a teams camp is ROADMAP phase 11.
  `tasks/` and `plans/` are not drawn: on this machine every `tasks/session-*` directory is empty
  and plans are slug-named files with no session link, so there is nothing to attach them to.
- **Harnesses.**
  Settled 2026-09-18: Claude Code only; there is no `~/.codex` or `~/.cursor` on this machine.
  The Codex adapter stays as the proof of the `harness.Snapshotter` seam, tested against a constructed fixture
  built from Codex CLI's documented rollout format
  (`~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` with `session_meta`, `turn_context`, `response_item` and `event_msg` lines);
  it is not trusted against the real format and a Cursor adapter is not planned.
- **Sprites.**
  Settled 2026-09-18 in [art-direction.md](https://github.com/Botropolis-City/meta/blob/main/roadmap/art-direction.md): Kenney City Kits, Nature Kit, Space Kit, Car, Train and Watercraft kits,
  pre-rendered from Blender to atlases by `tools/render-sprites`; workers are bots.
  Before that: milestone 5 shipped procedural shapes (vendoring third-party assets was Aria's call),
  then on 2026-09-17 three 16 px Kenney packs (Tiny Town, Tiny Factory, Roguelike Modern City),
  then the same day Kenney's 2D isometric packs for the isometric view.
  The rendered atlases replaced them all: the 16 px packs drew nothing once the top-down drawing path was deleted, and item 183 removed them too.
- **Stopped sessions.**
  Settled in milestone 5: catalogued by a head-and-tail window read over `projects/*/*.jsonl`, cached by size and mtime,
  aged by `parked_days` until item 86, which catalogues every transcript Claude Code still has.
  Since item 94, one active in the last day is read from the start of the plant's window on, so the plant's 24-hour figures are a whole read's.
  `sessions-index.json` was tried as a seed and rejected (cinders: 28 transcripts on disk, 4 indexed).
  Since ROADMAP phase 8 they lived in one storage district on the plan; since item 86 each keeps its own lot.
