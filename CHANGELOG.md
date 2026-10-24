# Changelog

All notable changes to Botropolis are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **The city holds about 38 MiB less graphics memory fully zoomed in, and 12 MiB less at every zoom.**
  The island's shore is painted coarser as the island grows, so its texture stays 1024 pixels a side, and the closest zoom draws a new middle level of the kit art in place of the largest one.
- **`status` names models short, as the city does.**
  The MODEL column reads `haiku-4-5` rather than `claude-haiku-4-5-20251001`, so one model no longer widens every row; `status --json` still gives the full id.
  The power breakdown's model mix drops the date too.
- **The strip's MCP chip says it counts calls**, as "6 mcp calls", so it no longer reads as six servers beside a map of two radio towers.
- **The help lists `f` and `0` on one row**, "f or 0: Skyline, the whole city", since both do the same thing.

- **The bridge runs off into the haze.**
  The deck no longer stops in open water: it runs on a few cells past where it ended and fades into the sea, and a delivery truck fades with it and is gone at the far end.

### Fixed

- **A finder row that is too long for the panel ends with "…"**, as the card and the sidebar do, rather than stopping mid-word at the edge.
- **The railway no longer shows through the trees.**
  Where the freight loop runs behind a tree, the tree now hides it; the trees look as they did over the grass.
- **A building looks the same at every zoom, however often it is drawn.**
  At some zooms, a building drawn again from what the city remembered of it could show its sign's letters or a boarded window's planks a pixel off from how it was first drawn; once it is wholly on screen it now matches exactly.
- **A delivery truck always leaves on the bridge.**
  When the island grew while a truck was on its way, the bridge moved and the truck carried on where it had been, beside the deck and over the water; it now takes the bridge as it stands.
- **The bridge stays where it is.**
  It moved over a row every time the island grew a row; it now keeps its place beside the same blocks as the island grows and across a restart, and moves only when its road does.

## [0.3.0] - 2026-10-23

0.3.0 is a performance release.
Measured on a laptop's integrated graphics (Intel Core Ultra 7 265U) with a real history of about 200 sessions in 150 projects, against 0.2.2 and the first builds of this release, measured the same week:

| | before | 0.3.0 |
| --- | --- | --- |
| drawing a frame, focused | 17–48 ms | about 10–11 ms |
| the city's memory, settled (own and GPU) | 715–817 MiB (4.6 GiB in 0.2.2) | about 445 MiB, 565 fully zoomed in |
| the city's heap in its first seconds | up to 3.1 GB | about 400 MB |
| a hidden city | 7.9% of a core and 462 MiB | under 1% and under 200 MiB |
| the daemon, settled | up to 24% of a core, in bursts of several cores | under 1% |
| the first launch | up to four cores, about 9 s to a map | under one core, a map in about a second |

Some budgets are still missed, and 0.3.x continues on them: a focused city uses about half a core where the budget is a tenth, and fully zoomed in it holds about 55 MiB more than its 512.

**Before you upgrade:**
- **Restart the daemon after installing** (`botropolis setup`, or the city's Restart Daemon dialog): a 0.3.0 city and daemon talk a new version of their protocol, and a city meeting an older daemon says so and offers to restart it.
- **Some commands moved**, and the old names still work through 0.x with a note on stderr: `tui` is `status --live`, `resume` is `attach`, `prune` is `rm --older-than`, `install-hooks` is `setup`, and per-setting flags such as `--daily_budget_usd` are `--set daily_budget_usd=5`.
- **`usage_refresh` is at least 15 minutes**: a config asking for less is read as `15m`.
- **Release builds no longer have the city's filming flags** (`--screenshot`, `--record` and the rest); `p` still saves a picture, and sharing mode records GIFs and clips.

### Added

- **[docs/compatibility.md](docs/compatibility.md) says what 1.0.0 promises not to break**, and `make version-check` now fails when any of it is removed without an entry here under *Removed* and, from 1.0.0 on, a new major version:
  every command and flag `--help` shows, `status --json`'s documented keys, `bar`'s JSON, every config key and its named values, every data file an earlier version wrote, and the package, release, unit and hook names.

- **Manual pages**: `man botropolis`, a page per command such as `man botropolis-attach`, `man botropolisd`, `man botropolis-hook`, and `man botropolis-config` for every config key with its default and environment variable.
  The deb, rpm, Arch package and Homebrew formula install them, and `make man` builds them.

- **Sharing mode, for posting your city**: `shift+s` in the city hides what identifies you (session titles, project and district names, prompts, spend and token figures, and every panel that shows them), then `p` saves a picture, `g` a four-second GIF at 1080p and 30 frames a second, and `c` a clip of 5 to 60 seconds (`l` picks), to your Pictures and Videos folders.
  The city wears neutral names while it is shared, such as District A and Session 3, and `esc` brings everything back as it was.
  A clip needs ffmpeg, which the deb and rpm suggest; the city says so when it is missing.

- **The reading pane (`o`) shows Claude's replies as formatted text, not raw Markdown.**
  Bold is bold, code and tables are set in Go Mono on a faint tint, a table with its columns lined up and its header in bold, lists have bullets and indent as they nest, quotes are indented, paragraphs have room between them, and a link shows its URL after its text.
  Your prompts and tool calls are shown as typed, and HTML in a reply is shown as text, never run.

### Changed

- **A frame of a large city takes a tenth of the drawing it did, and about half the memory.**
  Lines, lots, scaffolding, beacons, wires and glows are drawn as small meshes with a soft one-pixel edge instead of anti-aliased paths.
  On the metropolis demo that is about 75,000 indices a frame instead of 766,000, and a live heap of about 125 MB instead of 255 MB.
  Filled shapes read about half a pixel larger at their edges, and two of one colour that meet leave no seam between them.

- **A still city is drawn once and kept; only what moves is drawn again.**
  The ground, every building, tree, lamp, landmark and sign, the group lines and the map's names are put together once and reused until something in them changes, and the cars, drones, boats, trains and the rest are drawn again each frame only where they are, passing in front of and behind buildings as before.
  Panning with the arrow keys moves the picture and draws only the edge it uncovers.
  On the metropolis demo a settled frame is about 22 draw commands and 4,900 indices instead of 135 and 34,000, a pan about 29 instead of 138, and the city holds about 30 MB more of pictures at 1600×900.
  A scaffold's lamps, braces and hoist no longer swap corners as the map pans.

- **A still building is drawn as one picture instead of a dozen pieces.**
  Each building's walls, windows, sign, flags, scaffold and spire are put together once and reused every frame until its look changes; what moves, such as a pulsing beacon, smoke or a hoist's loads, is still drawn each frame on top.
  On the metropolis demo a settled frame is about 135 draw commands instead of 280, and the pictures take a third of a megabyte at the opening zoom and about 2 MB up close, within a 32 MB cap.
  A building's own sign, scaffold and spire now hide its lit windows behind them at night.

- **Zoomed all the way in on a large history, the city draws a frame in about half the time it did, with about half the CPU.**
  Buildings off the screen are no longer drawn at all, and those on it stay drawn as one picture each instead of being redrawn piece by piece once the city had more of them than it kept.
  The journal, the group review, the top strip, the event stream and the timeline are redrawn only when what they show changes, not every frame.

- **The city holds less in memory, most of all zoomed in.**
  Zooming in and back out no longer leaves the city holding pictures of buildings it has stopped drawing, the sprites load in smaller pieces, are packed onto fewer and smaller pages and sit in less memory, the picture kept for panning is let go once you stop, and the coast has its own texture rather than growing a shared one: about 135 MiB less at full zoom on a large history than early in this release, which is still about 55 MiB over the 512 MiB budget there.

- **An unfocused city keeps up, as a dashboard on another monitor.**
  It used to show the last frame it drew while it had the focus; it now draws again whenever something changes, such as a session starting, stopping or needing you, and nothing moves in between, so it costs little while it sits there.

- **A city you cannot see costs far less while sessions work.**
  An update from the daemon is now laid out in milliseconds rather than more than a second on a history of a few hundred sessions, and is laid out at all only when it changes the map.
  A hidden or minimised city lays out nothing until you bring it back, and its window title still says at once that a session needs you; on 200 sessions it went from about 6% of a core to under 1%.

- **The daemon sends the city only what changed.**
  A city used to be sent the whole picture of every session on every update, about 260 KB a second while 200 sessions worked; it now gets the whole picture once and then only the changes, about half a kilobyte a second.
  An unfocused city decoding and laying out those updates spends under 1% of a core on it.
  A city and a daemon must still be the same version.

- **A city left open on another monitor costs about a thirtieth of what it did while your projects share work.**
  Every update from the daemon used to lay out every street between projects again, which on 200 sessions in 150 projects with 80 streets between them took about a quarter of a second, and kept an unfocused city at over a fifth of a core while one session worked.
  The city now lays out a street again only when the plan, the camera's heading or the projects it joins change, and an update that moves only what sessions are doing takes a few milliseconds: under 1% of a core.

- **A city on another monitor draws each change for a third of the CPU it did.**
  The cracks and wear on long-quiet lots are worked out once rather than every frame, and the ground under the city is no longer redrawn for an update that changes nothing on it.
  On a real history of about 200 sessions with two working, drawing fell from about 1% of a core to 0.3%.

- **With statistics on, a city left open while sessions work no longer spends a sixth of a core reading your costs.**
  Its 24 h cost was read from the statistics store every 2 seconds, and each building's every 30, which on a history of 30,000 messages kept the city at about 16% of a core; it now stays under 1%.
  The store keeps a note of what each write changed, so the city reads again only those messages and prices again only those sessions, rather than reading the whole day and every session's history.
  The 24 h cost is brought up to date at most every 30 s while sessions work, and every 5 minutes when nothing changes, so it can lag by up to 30 s (or 5 minutes for messages leaving the 24 h window); a building's cost is brought up to date only when something was written.

- **The command line is smaller: 13 commands in four groups, and four global flags.**
  `botropolis --help` groups the commands as sessions, the city, setting up and checking, and those other programs run.
  `attach` attaches in the terminal you are in, and opens one only when run from elsewhere or with `--new-window`; given a resting session it resumes it first, so a plain `claude` no longer drops to `claude attach`.
  `rm` takes several ids, or `--older-than` for every resting session quiet that long.
  `status --live` is the live table.
  `setup` registers the hooks, starts the daemon with systemd or Homebrew and says exactly what it ran, and prints the line for your shell's startup file.
  `shell-init` loads completions too.
  `--set key=value` sets any config key for one run, including those that never had a flag, so `--help` on every command no longer lists two dozen city settings.

- **Release builds leave out the city's frame tooling**: `--screenshot`, `--record`, `--fps`, `--seconds`, `--keys`, `--hover`, `--script`, `--headless`, `--window` and `--signage`, which exist to film the city and test it.
  `p` still saves a screenshot.
  `make build-dev` builds them in, with the `devtools` tag.

- **`usage_refresh` fetches usage at most every 15 minutes, and only while you can see the city.**
  Each fetch runs Claude Code, which takes up to 45 seconds and a few hundred MiB, so the old `5m` spent up to a sixth of the time on it; `5m` and anything else under 15 minutes now means `15m`, and the settings menu offers manual, 15m, 30m and 1h.
  A hidden or minimised city fetches nothing on its schedule, and `u` still fetches whenever you press it.

- **`botropolis bar` leaves out the sessions you set aside in triage.**
  A session marked done with `d` and quiet since, or snoozed until a time still to come, is no longer named, listed in the tooltip, or counted as needing you; the bar counts it as "set aside" instead, as in "○ 1 working · 2 set aside", and the line moves on to the next session the way the city's footer does.

- **The city counts set-aside sessions the way the bar does.**
  The top strip, the window title, the needs-you chip's card and the Attention lens no longer count a session you marked done or snoozed as needing you; the strip shows it in a plain "1 set aside" chip, and clicking "needs you" skips it.
  When the snooze runs out it needs you again everywhere at once (the window title at the next update from the daemon).
  Its counts now read as the city's do, "2 need you" rather than "2 needs-you".
  The JSON's keys and classes are unchanged.

- **At night the landmarks light their windows too.**
  The library and City Hall are lit all night, and the power plant while any session is working, each from its own windows, warmed a little yellow; by day they look as they did.
  The power plant's round glow at night is gone: its spend is on the strip's "$… 24h" chip and in the Cost lens.

### Deprecated

- These keep working through 0.x, each naming its replacement on stderr, and are removed at 1.0.0: `tui` and `--tui` (use `status --live`), `resume` (use `attach`), `prune` (use `rm --older-than`), `install-hooks` (use `setup`), `version` (use `--version`), and every per-setting flag such as `--daily_budget_usd` (use `--set daily_budget_usd=5`).

### Removed

- **The `projection` config key and `--projection` flag.**
  The map has been isometric only since the top-down view went, and the key only opened the isometric map with a warning.
  A config file that still sets it loads as before, and the key is ignored.

### Fixed

- **The first launch no longer takes the machine.**
  Reading your whole history for the first time used up to four cores; the daemon now reads it at most half the time, at a lower CPU priority on Linux, so it stays under one core in all.
  On macOS its disk reads now wait behind everyone else's; its CPU priority stays where it was there, because every lower one measured froze the whole daemon on Apple silicon.
  The first read takes about twice as long, and nothing else waits for it.
  Quitting part-way through it now loses at most 16 MiB of one transcript, where it used to start that transcript over, however long.
  While it reads, the footer shows how far it has got on a bar, as in "Reading your history · 42% · about 6 min left", with the time left once its pace has settled.
  A city opened while the daemon is still reading your sessions no longer shows an empty island: it shows "Reading your sessions · 40 of 120" on a bar until the map is ready.

- **The map comes up within about a second of the daemon starting, on a large history too.**
  Every start used to read your live sessions and the last day's whole before the map could draw, which on a 3 GB history took about 5 to 9 seconds at a full core.
  The map now draws first, from the start and end of each session, and the rest of the day is read behind it at half a core, with "Reading your history" in the footer until it is done, so a session's usage and the power plant are marked as still filling in rather than shown low.

- **The daemon no longer costs more the more sessions you have ever had.**
  With statistics on, a home of 2,000 sessions with one of them working used about 45% of a core; it now uses about 2%, because each time something changes it reads only what changed rather than every session again.
  On a home that large, token counts and the power plant can now trail a session's work by a few seconds, never more than 5; a session's state still changes the moment Claude Code says so.

- **The daemon gives back the memory it used to start up, even when no session is working.**
  It used to return freed memory only after a hook event, so a daemon that read your history and then saw no hook kept its start-up peak; it now returns it once the reading has been quiet for 5 seconds, and at least once a minute while sessions keep it busy.
  On a home with 2,000 sessions it settles at about 42 MiB, where it held about 60.

- **The daemon no longer spends most of its CPU collecting garbage.**
  It capped its own heap at 16 MiB and forced a collection after every rescan, so a session writing steadily cost it about 290 collections a minute; it now makes about 20, and uses about half the CPU.
  Its heap limit is now 256 MiB, and `GOMEMLIMIT` still overrides it.

- **The daemon no longer reads every session's statistics again whenever a transcript changes.**
  The statistics database now records which sessions' counters a write changed, whichever program wrote it, and the daemon reads only those; on a 3.2 GiB history that was about 85 ms of CPU for every line a session wrote.

- **The daemon rescans at most once a second, and no longer looks for every transcript again each time.**
  Each line a session wrote was its own rescan, and every rescan globbed every project; now changes within a second are read together, the first after a quiet spell still within a tenth of a second, and directories are listed again only when they change.
  Hooks still change a building at once; only token counts and the power plant can lag by up to a second.

- **The daemon no longer compares every pair of sessions again on every prompt.**
  Grouping keeps each pair's score and scores again only the pairs a prompt can have changed, and the prompt history is read from where it was last read, rather than parsed from the start twice on every load; with 100 sessions on the map, a prompt costs the daemon about 20 ms of CPU, not 150 ms.

- **A prompt costs the daemon less on a long history.**
  With 2,000 sessions in the last 30 days, the daemon's work after a prompt takes about 15–20 ms of CPU, where it took up to about 50, because grouping now goes back over only the sessions the prompt changed.

- **A long history no longer fills the daemon's memory with grouping.**
  Grouping kept a score for every pair of sessions from the last 30 days, about 415 MB with 2,000 sessions, past the daemon's 256 MiB limit; it now keeps only the pairs that have something in common, about 3 KB a session.
  A prompt among 2,000 sessions now costs grouping a few milliseconds rather than more than a second.

- **Refreshing usage no longer starts your MCP servers, settings, hooks or plugins.**
  Each refresh used to start Claude Code with everything you have configured, up to seven processes and about 900 MiB; it now starts Claude Code alone, at under half that.
  A refresh and a summary also never run at the same time any more, and two summaries run one after the other.

- **The city opens on the sessions already running, standing, instead of sailing them in.**
  With statistics on, which is the default, every live session arrived by tug when the city opened and stood as a bare lot for its first seven seconds before being built.
  The same start-up replayed what had already happened: subagent drones took off, every open pull request's flag was hoisted, every merged one was celebrated, and a van went out for every message already sent between projects.
  Now only what changes after the city opens is shown happening.

- **At night a building's own windows are lit, not round halos over them.**
  Each lived-in building keeps its windows at their daytime brightness while the rest of it goes dark, so a lit building reads as its windows rather than a grid of discs.
  Boarded-up windows fit every building too: an industrial building's planks no longer hang off its corners.

- **A new building's windows light floor by floor as it rises.**
  At night, a session still arriving by tug lit every window of its finished building over its empty lot, as rows of floating discs, and a building going up glowed above its own rising roof.
  Now nothing is lit before the building stands, and each storey's windows light once that storey is built, so a building lights from the ground up while the storey going up stays dark.
  A building's lit windows also sit on its own storeys now, so a new storey lights as soon as the building gains it.
  Under reduced motion they fade in with the building.

- **At night, lit windows no longer show through what stands in front of them.**
  A district's sign, a building's own painted name, its beacon and flags, and a scaffold's tubes and boards now hide the windows behind them, as a building in front always did; something half transparent dims the light behind it rather than hiding it.

- **A road or a tower's beam you point at no longer stays highlighted after the pointer moves off it.**
  Moving from one road or beam to another, onto bare ground, or from a selected building to its neighbour while its roads are shown now lights only what the pointer is on.

- **The minimap stays in its panel after the camera turns, and turns with the city.**
  After pressing `r`, the minimap's map was drawn off its panel and down over the footer, and it showed even when the whole city was already in view.
  Now it sits in its panel at every heading, facing the way the city does, and stays hidden while the whole city is on screen.

- **The live table (`status --live`) scrolls when there are more sessions than lines.**
  Its column names and the selected row stay on screen, and a line under the rows says how many lie above and below.

- **The live table (`status --live`) no longer breaks text with accents, dashes or non-Latin scripts.**
  A shortened title, folder or tool line could end in a replacement mark, lines stopped short of the terminal's edge, and a title in a wide script such as Chinese or Japanese pushed the columns after it out of line.

- **Three lens panels no longer read as wrong beside the strip.**
  Attention no longer tells you to press tab when nobody is waiting, and tab then says "nobody needs you right now".
  Tab and the footer say "needs you" rather than "needs-you".
  Work counts groups in the cards' words, such as "1 of 2 groups are your move".
  Cost says its total is since the sessions started, and "up to" when any spend is a range.

- Counts agree with their numbers: the strip, the window title, the sidebar and the City Plan say "1 needs you", "1 PR", "1 error" and "1 subagent" instead of "1 need you", "1 prs", "1 errors" and "1 subagents".

- **`botropolis bar` names the session that has waited longest, not the one first in the alphabet.**
  Its tooltip lists who needs you in the same order, longest-waiting first, which is the order triage (`w`) walks them.

- **The key reference (`?`) lists `u`, and names every key as you have set it.**
  It left out `u`, which fetches the plan's usage again, and always showed triage, the journal, the stream, New session and explain at their default keys, however they were rebound.

- New session names the session its group guess is based on by its title, as the board does, rather than by its id.

- **Your statistics hold only your own sessions.**
  A daemon started with `--home` on another Claude home no longer records that home's sessions into your statistics, as it already kept out of your ledger and groups.
  Running Botropolis's own tests no longer adds a test session to them either.

- Codex sessions can be packed up and unpacked like Claude's, and packing one no longer leaves a ghost in the depot.

## [0.2.3] - 2026-10-19

### Changed

- **The Botropolis city and its daemon must be running the same version.**
  After an upgrade the old daemon keeps running until it is restarted.
  The city no longer loads beside one of another version: it shows both versions with **Exit** and **Restart Daemon**, which restarts the daemon the way it was started — its systemd user unit, its Homebrew service, or the process you started by hand — and opens the city once it answers.
  When the daemon is the newer one, the button is **Restart City**, which runs the city again from the upgraded build.
  `status`, `events`, `find`, `bar`, `notify` and `tui` refuse with one line naming both versions and how to restart the daemon, and `botropolis doctor` fails its daemon check the same way.
  Claude Code's hooks are never refused, so a tool call is never blocked.

- **A train's engine shows its model's context window.**
  Each window size has its own engine: a diesel up to 200k tokens, a bullet train up to 1M, a double-decker over 1M, and a steam engine when the window is unknown.
  The wagons still count the model's spend.
  Hovering a train shows its window and where it was read: from the model's sessions, from the model table, or unknown.

### Fixed

- **Your district layout is never thrown away when it cannot be read.**
  A layout file written by a newer Botropolis is refused by name, with a message to update, and left as it is, rather than replaced by an empty layout;
  a layout or session ledger from an older version is carried forward.

- **The map uses far less memory starting up and once running.**
  The city's art used to be held twice over while it loaded, and its pages were packed into larger textures than they need.
  It now loads a page at a time once the window is open, each page on a texture of its own size: on a software renderer the start-up peak fell from about 1.3 GB to under 0.5 GB, and what the city keeps from about 0.9 GB to 0.5 GB.
  The window shows its background for a moment longer before the city appears.

- **Trains run nose first.**
  On stretches of track running one way across the map they were drawn backwards; the old diesels looked the same from either end, so it never showed.

- **Cars, vans and trucks drive nose first.**
  On roads running one way across the map the traffic, the message van, the delivery truck and the depot's truck were drawn driving backwards,
  and the skip trucks and cement mixer were backwards on roads running the other way.
  Every vehicle, the tugs included, now turns by one rule, and the tugs keep their pointed bow forward.

- **Cars keep to the right.**
  Every road is two-way, and the traffic, vans and trucks drove down its middle, so two going opposite ways passed through each other.
  Each now drives in the lane on the right of its travel, round corners, through junctions and over the bridge,
  and the inspector's car and the site trucks turn back across the road rather than reversing.

- **A long-lived session no longer slows the city down every time it writes.**
  The daemon read a running session's whole conversation again on every change, so a session with a few hundred megabytes of history cost seconds of CPU per write, and the map was late to show what sessions were doing.
  It now reads only what was added since it last looked, for each session and each of its subagents, and reads the file whole only when it was replaced or rewritten.
- **The bridge road climbs a gentle ramp instead of a steep solid wedge.**
  The ramp starts at the junction and rises over a cell and a half on short piers, open underneath like the rest of the bridge, before crossing the railway.
  The railway's west side now runs one cell further out to make room for it.

## [0.2.2] - 2026-10-19

### Fixed

- **A session you demolish stays off the map even when its conversation is still on disk.**
  `claude rm` deletes the background session and its worktree but keeps the conversation in Claude Code's files, and 0.2.1 rebuilt a resting session from it.
  The daemon now remembers what you demolished, across restarts, and leaves it off the map and out of `botropolis status`; if you resume the conversation yourself, it comes back.
  The demolish card now says what happens: the session and its worktree go, and the conversation stays in Claude Code's files.

## [0.2.1] - 2026-10-19

### Fixed

- **The daemon starts however large your Claude Code folder is.**
  0.2.0 read the whole folder twice before it counted as started, and on a large one on a busy machine it gave up after 10 seconds with `context deadline exceeded`.
  It now starts as soon as its socket is open and reads the folder after: the city and `botropolis status` wait for the first read and never see a half-read city, and Claude Code's hooks are answered at once, so no session waits on it.

- **A session you demolish leaves the map for good.**
  `d d` on the map, the TUI's demolish, `botropolis rm` and `botropolis prune` now have the daemon forget the session, so it no longer comes back as ruins once its transcript is gone; with no daemon running, the daemon forgets it when it next starts.
  A session whose transcript Claude Code cleaned up on its own still keeps its lot, and a removed session's statistics are kept or pruned as before.

## [0.2.0] - 2026-10-19

### Added

- **The power plant shows your usage over months, not only hours.**
  Beside 1 h, 24 h and 7 d, its panel offers month to date, last month, the last 30 days, 3 months and 12 months, year to date and a custom range, read from the statistics store; ← and → step through them all.
  Each range draws a bar a day or a month, the split by model and speed, the split by district, and this month against last, with `s` stopping last month on the same day of the month.
  `d` chooses which districts count, all by default.
  The panel says how far back the data goes, draws days before it differently from days with nothing, and marks a stretch when statistics were off as possibly incomplete, on a range and on the comparison.
  While statistics are off it offers only the short windows and says so, and `s` opens settings on the statistics row.

- **The reference thanks the people whose work the city is drawn with.**
  The page `?` opens on ends with a credit in small print: the roads, trees, vehicles, trains, boats and landmarks are Kenney's (kenney.nl), under CC0,
  and the type is Inter, by The Inter Project Authors, under the SIL Open Font License 1.1.

- **The City Plan: every district on one screen.**
  Zooming out stops at Skyline; one more notch, once the wheel has rested, turns the city into a flat plan.
  Each district is a badge at its place on the map, with its name, whose move it is and its counts of sessions, of those needing you and of subagents in flight; the groups' lines run between them, and a legend reads the usage limits.
  Zooming in returns to the map at Skyline, with the district under the pointer still under it.
  A few dozen directories fit: as the plan fills, every badge shrinks to its name, a dot in the tone of whose move it is and how many need you, and no two badges overlap; one that needs you keeps its own place and the rest make room.
  `m` (`city_plan_key`) opens the plan from anywhere and closes it back to where you were, as does Esc; clicking a badge flies into its district on the map.
  The plan is drawn as a map: the city's land lies inside an inked coast that wanders about a cell out from its lots, in a wash of water, each lot is tinted faintly by whose move it is, and the plaza is a small civic mark that no badge covers.
  The roads between districts run along their streets as dashed ink, full while travellers are working on them and faint while they are quiet.
  A compass in the top-right corner points north as the plan lies and turns with it on `r`, and a title block in the top-left reads *City Plan* and the time on the city's clock.
  Badges, the compass and the title block keep clear of the lens panel and the stream's corner, which the plan works round rather than giving up the band beside it, a hovered badge's card is raised beside it, and the minimap is put away while the plan is open.
  The top-down projection is no longer offered: `--projection top` and a config that asks for it open the isometric map, with a warning.

- **Statistics are kept, so the scale outlives the transcripts it was cut from.**
  Botropolis keeps each assistant message's time, model, directory, thread and token counts in a SQLite database of its own, `stats.db` in its state directory, and never anything that was said.
  A message a fork copies is counted once and belongs to both sessions.
  The scale is cut from every stored session, including those whose transcripts Claude Code has since deleted, and a restart starts on it at once.
  While statistics are on, the daemon reads only what is new in each transcript instead of reading stopped ones whole.
  `botropolis prune` deletes the statistics of the sessions it removes.
  The `stats` setting, or the statistics row in settings, turns it off.
  Turning it off asks whether to keep what is stored or delete it,
  and turning it back on over kept data asks whether to keep it, marking the gap, or start fresh.

- **A building's height is how much work its session has done, on your own scale.**
  A session's heft is the tokens it has sent and received over its whole life, subagents included, with cache reads counted at a tenth.
  Your sessions are cut into fifths by heft, so a building stands one to five storeys and a typical session of yours sits at three; the scale is recomputed at most once a day, and no building ever loses a storey.
  The daemon reads stopped sessions' transcripts whole in the background after the map is up, so startup does not wait for it.
  Each height is its own kind of building, so floors can be counted at a glance: a cottage, a family house, townhouses, and apartment blocks.
  Every district has a type, residential, shopping, office or industrial, and its buildings are that type's kinds: shops, glass offices and workshops through works yards grow the same way.
  A new district's type is chosen by the `district_type` setting: `random` (the default, the same for a directory every time), `guess` (from the names at the top of its directory, never its contents), or one type for every district.
  A district keeps its type once it has one, and a building's card changes it.
  A session heavier than where your five-storey buildings begin raises a thin spire from its roof: one segment at twice that line and one more at each doubling, up to six, with a red beacon at the tip; like a storey, a segment once reached stays.
  Mid-turn, the storey being built is scaffolded on the roof, one storey high on the building's own footprint, and each tool call's load goes up the hoist to it.
  Each compaction slides a course of the building's own material under it, up to three, stepping in like steps up to its door, the newest lit for a moment; past three, the bottom course carries the count.
  How full a live session's context is stands at the kerb beside its building as a skip: there from half its window, filling as context rises, heaped from 80% and overflowing near the limit, with the exact figures on its card.
  On a compaction a skip truck hooks the full skip and drives off, and as it leaves a cement mixer pulls up and pours the new course under the building; at still the skip just empties and the course is there.
  A session whose transcript Claude Code has deleted stands as the ruins of its building, abandoned and fallen apart in its district's material, with its lot and its card as before.

- **The city moves to say what changed, as much as you choose.**
  A `liveliness` setting (still, calm, steady or lively; `--liveliness`, and `--reduced_motion` means still) sets how much the city moves; at still every mover stays on the map where it is and changes cross-fade.
  A new session's building is built on its lot when its tug docks: fence, scaffolding and crane, then the floors, then the lights.
  While a session is mid-turn its roof is scaffolded with work lights, each tool call runs a load up a hoist on its corner, and the building's card names the tool; the rover at the door is gone.
  Each subagent in flight is a small drone at work on its session's building, or on a building in the project it works in, with a shadow beneath it, counting agents launched in the background too.
  A pull request hoists a flag and parks an inspector's car outside while it is open; a merge sends a crate off the island by the new bridge, and the merge that leaves a group with nothing open cuts its ribbon.
  Each message one session sends another, in a team or directly, drives a van along the road between their projects, or runs a carriage along their group's line.
  The map zooms out no further than where buildings still read, and `f` is now Skyline: the whole city if it reads there, otherwise that furthest zoom, centred.

- **The footer names the session that has waited longest for you.**
  While a session needs you, a line above the key row reads `needs-you: <its title>`, with `(+N more)` when others wait too, in the order triage walks them.
  A click on the line attaches to that session; a click on a building still only selects it.
  A status message draws above the line rather than over it.

- **Groups: sessions across directories that are one piece of work.**
  Botropolis groups sessions by what they already say and how they were started: a shared ticket key, pull-request link or `Depends-On:` line, agent teams and sessions that message each other, the same pasted text, rare words in common, a shared branch key or files, and sessions started or continued close together.
  Nothing is tagged: each join says why, a guess is marked as one, and prompts are read where Claude Code keeps them and never copied.
  A group is named after the reference its members share and the words more of them use than your other sessions do, in the order they were said, so common words such as "allow" or "through" never name it.
  A member wears a badge with its group's short name and a dot in the tone of whose move it is (yours, the agents', someone else's, or nobody's), and its card names the group and why it joined.
  Each group is drawn as a line across its districts, with a station in each in the order its work started; a group lines setting hides them.
  `g`, or a click on City Hall, opens a board of every group with what happened last time, what is next, whose move it is and how long it has waited, and Enter focuses the map on one.
  Move to group on a card, or dragging the selected building onto a station, moves a session between groups, and the correction is remembered.
  The first launch proposes the groups found in the last 30 days on one screen, where they can be kept, merged or split, until you confirm or ask to be asked later.
  `Shift+N`, or the board's button, opens New session: a directory completed with Tab, a prompt, and the group guessed from both, which the session starts in.
  Groups are kept in `~/.local/state/botropolis/groups.json`, for your own home only.

- **A focused group docks a list of its stations, and its line reads their order.**
  Focusing a group, from its line, its board row or the finder, docks a list beside the map on the side away from the river: each site's sentence, its newest pull request, whose move it is and how long it has waited.
  `j` and `k` step through it and centre on each station, Enter reads the site's session in the reading pane, and Esc leaves.
  The finder finds groups by name too, after the sessions it found.
  `c` on the board, or copy stand-up on a member's card, copies one line saying where each site stands, such as `lib merged; api in progress; infra in review, seen 3 days ago`; an open pull request older than a day says when it was last seen, because Botropolis knows it only as a session last saw it.
  A line puts its stations in the order they can be read to depend on, from a `Depends-On:` footer, a pull request named beside "once" or "after", a version bump another site tags, or a recap waiting on another site, else in the order they started.
  A site that waits is drawn unlit until what it waits on is seen, a merge or a tag made by a command a session ran, and its card and row say what unblocked it.
  A pull request the group's sessions name but none of them opened is a grey station; it is merged or closed only when a session ran the command that did it.
  A session whose subagents worked in the group's other directories, or which messaged sessions there, is its line's site office, drawn as a square at the head of the line, and grouping counts it above every other signal; messages are matched to sessions by name, and a name two live sessions share counts for neither.
  A session joined only by its wording offers mute a word on its card, so that word stops joining sessions.

- **Find a session by what was said in it.**
  `Ctrl+K` (`⌘K` on macOS) opens the finder, a panel inside the city that searches the prompts and replies of every session, with no tokens spent.
  Each result shows the words it matched in context, and the map dims every session the finder did not find.
  Enter resumes the chosen session the way its card does, `Ctrl+Y` copies `cd <dir> && claude --resume <id>`, and `Ctrl+M` closes the finder and shows the session on the map.
  The three chords are settings (`find_key`, `find_copy_key`, `find_show_key`).
  `botropolis find` opens the finder in the running city, or starts the city with it open, so a desktop's own keyboard settings can bind it; `botropolis find -n 5 "words"` prints matches in the terminal, with or without the daemon.

- **Read a session inside the city.**
  `o`, or *read* on a session's card, opens a pane over the map with its last turns: your prompts, Claude's replies, and one line for each command run or file touched.
  `PgUp` and `PgDn` page back and forth, reading no more of the transcript than the pane shows, Enter opens its terminal to reply, and Esc closes it.
  A session waiting on you leads with its question and its options word for word, or with the tool call it is waiting to run and the hook's note.
  When Claude Code has deleted the conversation, the pane says so, lists the prompts you sent, each with a copy button, and Enter offers a new session in the same directory.

- **A global shortcut for the finder, off until you set one.**
  The `global_shortcut` setting, recorded from the settings panel, opens the finder and brings the city's window forward from any application while the city runs.
  A combination that commonly means something elsewhere, such as `Ctrl+K`, is named before it is saved; one another application already holds is refused, with the fallback; Backspace clears it.
  It works on X11 for now; on macOS and on Wayland the row says the desktop has no way to register one yet.
  `--config` points the city at another configuration file.

- **The map explains itself.**
  A session's card leads with a sentence: the question it is asking you, else Claude's recap, else a generated summary, else what you last asked; then its group and station, three numbers for the current lens, and its actions, with *details* folding out the full table.
  Six lenses, on `1` to `6` or from the list `v` opens, each change what the map colours and what a card leads with: Attention, Work, Cost, Capacity, Health and Connections.
  Each lens's panel says what it shows, gives one number and what to do about it, and each card's first action follows the lens; Capacity's is *fork a fresh session*, beside *new session here*.
  Every panel and card says in one line what it is, and every button says what it does when the pointer rests on it.
  Holding `e` (`explain_key`) captions one of each kind of thing on screen, `?` opens a reference of everything on the map and every key, whose terms open each other, and the first time a kind of thing appears on a machine it is captioned once, one kind at a time.

- **Triage, the stream, and district signs.**
  `w` (`triage_key`) walks the sessions waiting on you, longest-waiting first, flying to each: Enter acts, `p` rests it, `d` marks it done, `s` snoozes it until a time you pick or type, `n` moves on.
  The stream shows what happens newest first, with what needs you in amber and errors in red and everything else muted; a click flies to the session, and `l` (`stream_key`) hides it.
  It is a panel in the top-right corner about a card's height, at most a quarter of the window wide, wrapping its entries to fit, and in a window too narrow for it, it stays hidden until `l` shows it.
  The wheel scrolls it through the history while the pointer is over it, as does dragging its scroll bar; scrolled back, it holds still as new entries arrive and says how many are newer above.
  Each district stands a sign at its corner with its name and whose move it is, in place of the floating name plate wherever the sign is legible.
  Zoomed in, the board stays the size it reads at and its post grows instead.

- **Drag a district to another slot.**
  Press on a district's floor or name sign and drag it; the slot it would land on is outlined, and it stays where you drop it.
  Dropping it on another district swaps the two, and the ring just outside the city takes it too.
  Hold Shift or Alt to pan instead, wherever you press.

### Changed

- **Usage is counted one way everywhere, and its cost is the list price of each counter.**
  The power plant, its trains and the resource strip read the statistics store, each message under the model that answered it, and price it at Anthropic's API list prices for that model: input, 5-minute and 1-hour cache writes, cache reads and output each at their own rate, fast mode, US-only inference and long prompts included, and web searches per use.
  A cost is labelled "at API list prices" with its messages beside it, and the counters are listed apart under it, never added together; cache writes whose lifetime a transcript did not record make the cost a range.
  The plant used to show "tokens" that added cache reads to output and pro-rated each session's lifetime cost across the window, and it credited a session's whole usage to the model it was using last.
  Fresh tokens are now input and cache writes, without output, which has a chip of its own, so the hit rate is the share of prompt tokens read from the cache.
  A train's wagon stands for dollars rather than tokens.
  While statistics are off, or the store cannot be read, the plant gives no dollars and no split by model, the trains stay in the yard, and the strip says why in a chip of its own.
  Claude Code 2.1.286's own figure prices Sonnet 5.5's cache reads at the rate before Anthropic's cut of 7 October 2026; Botropolis prices each message at the rate in force when it was sent.
  Codex sessions no longer add to the plant, whose counters and prices are Claude's.

- **The power plant, the library and city hall face you, with their names on their walls.**
  All three are turned round, so their front doors face the camera's home heading, and the plant's chimney turns with it.
  Each one's name is painted in capitals on a clear stretch of one of the walls you can see, leaning with the wall and turning with the camera, never across a door or a window, and moved to the part of the wall still in view, stacked if it has to be, when another landmark stands in front.
  Their floating labels are gone: a landmark's plate shows only while the pointer is on it.

- **The packages declare every licence they ship.**
  The deb, rpm, Arch package and Homebrew formula now declare `MIT AND OFL-1.1 AND CC0-1.0`, not MIT alone, because the binaries carry the Inter typeface and Kenney's art as well as botropolis's own code.
  Each package already installed all three licence texts; now its metadata says so too.

- **A road runs from one lot to the other.**
  Every lot has one driveway, paved with a short pad into the lot while one of its roads is in use, and a road between two districts runs from one driveway to the other along the streets, instead of along an avenue beside both lots and into neither.
  A lot's driveway goes on the side of its block that keeps its roads shortest, the side facing the camera unless another is more than two cells shorter, so two lots across one avenue meet across it.
  It is chosen when the lot first has a road and kept from then on: new roads, turning the map and restarting leave it where it is.
  Cars, message vans, the merge truck and the compaction's skip truck and mixer all come and go by the driveway, and a road that reaches the bridge joins it with a junction.
  Pointing at a car lights its road, driveway to driveway, and its two districts, with their group-line stations, and fades everything else, labels included; the road's tag stands clear of the car.
- **Trees are scenery, and look it.**
  The woods between districts grow in natural clusters with open ground between them instead of one tree to a cell, they are drawn quieter than buildings and everything else you can hover, and the ground under them no longer shows a card.
- **The city stands on an island.**
  The map no longer ends at a cut: the plan stands on land with a soft, irregular coast, a beach and shallows, in a sea with a faint far shore, and the island grows as the city does.
  The river runs out to the sea at both ends between banks that wander like a river's, opening into the sea and fading into its shallows, and arriving and departing sessions' tugs sail in and out from the sea.
  You can pan far enough to see the coast all round.
- **Roads are laid only where two districts are connected.**
  The ground between districts is grass, and a road appears on it when a session reads or edits files in another project, messages a teammate working there, or has a subagent work there.
  Each pair of districts has one road, whichever way it was crossed, and its card shows the traffic both ways.
  The road stays while any session that made the connection is on the map, and grows over when the last one goes.
  Lamps and street trees stand only along roads.
- **A car is one session working across a connection**, so the number of cars on a road is the number of sessions working across it right now, and a road that nobody is working on has no cars.
- **The trains run on the main map, as the cars do.**
  The token trains round the loop are no longer drawn only by the Cost view: they run on the map the city opens in too, and pointing at one shows its model, its tokens, what a wagon stands for and the cost.
  A day with no token spend still has no trains.
- **A stopped session stays on its own lot, and the storage yard is gone.**
  A session whose process has stopped is *resting*; after `shuttered_days` (7) quiet it is *shuttered*, and after `overgrown_days` (30) it is *overgrown*.
  Its card names the rung and says how long it has been quiet, and a district stays on the map while it has any session at all.
  The state once called `parked` is now `resting` everywhere, in `botropolis status --all`, its JSON and the TUI.
- **Nothing leaves the map on a timer.**
  Every stopped session Claude Code still has a transcript for is shown, however old.
  The daemon keeps a ledger of the sessions it has shown — ids, directories and times only, in `~/.local/state/botropolis/sessions.json` —
  so a session whose transcript Claude Code has deleted keeps its lot, titled from your own prompts, and its card says the conversation is gone.
- **Each rung is drawn.**
  A stopped building's windows go dark, with no beacon or sign; a shuttered one has planks nailed across each of its own windows; and its lot ages: the paving wears and cracks, and an overgrown lot grows grass in the cracks, tufts at the base of the walls and in the yard, and now and then a sapling.
  When an overgrown session wakes, its lot is mowed in 1.5 s, or cross-faded in 0.3 s under reduced motion.
- **A district ages as a whole once every session on it has.** A district takes the freshest rung among its sessions: when all of them are resting its whole floor wears, when all are shuttered it cracks, and when all are overgrown it turns mossy and grows over edge to edge. One live session keeps it as it is. An overgrown building grows along the whole of its front.
- **Packing up and unpacking are drawn.** A truck carries the crate between its lot and the depot over 2 s; under reduced motion the session fades in where it lands.
- **Pack up and unpack.**
  *Pack up* takes a session off the map and keeps it as a crate in the depot along the city's south edge, a row per project, oldest first; *Unpack* brings it back to its old lot.
  Both are on a session's card, with a one-line description of what each does, and on Shift+P; `d` in triage packs the session up.
  A stopped session whose pull request has merged is packed up too, and a packed-up session that does something new comes back by itself.
  What is packed up is kept in the daemon's ledger, which is why packing needs the daemon.
  The depot fits its crates, a row per project and as wide as the longest row, and is not there at all when nothing is packed up; its sign says how many crates it holds.
- **Close up.** An optional way to end the day: Shift+Q, or the button beside the quit prompt, notes with the daemon when you closed up, which sessions were open and which you pinned to start with next time, so coming back picks up where you left off.
  Every live session is left running unless you mark it *pause*, which keeps its conversation; a session mid-turn says pausing interrupts it, and one open in a terminal stays running. With nothing live it records at once. It spends no tokens, and quitting without it changes nothing.
- **Coming back.** Opening the map after a gap (journal_hours, 8 by default) opens a journal of where you were: one entry per group and per session in no group, pinned ones first, then most recently touched, each with what it was doing last time and what is next, from Claude's recaps or your last prompt and its reply, read then and never kept.
  Above it, a strip of what finished, failed and needed you since you left, and any usage window that reset; beside it, *Restore the desk* (or R) reopens in the background the sessions that were open when you left. Enter goes to an entry, Esc to the map, and Shift+W (journal_key) brings it back.
- **Codex rollouts are read through a 64 KB head and tail**, as Claude's transcripts already were, so a long Codex history loads in a time bounded by the number of rollouts rather than their size.
  They are read in parallel, and only the lines the map uses are decoded, so a daemon starting on thousands of them is up in well under a second rather than several.
- **`status --json` marks usage it read only part of.**
  A session whose transcript was read through the window carries `usage.partial: true`, so its counts are not taken for totals.
- **A card's buttons light up under the pointer**, as the footer's needs-you line does.
- **The top-down projection draws the island in its own palette**, with the plan's ground and river on it, instead of a darker rectangle with the river in square cells.

### Removed

- **`parked_days`**, its flag and its settings row: nothing is taken off the map by age any more.
  A config file that still sets it loads as before, and `shuttered_days` and `overgrown_days` replace it in the settings panel.

### Fixed

- **The city keeps up with your sessions while the statistics are caught up.**
  The first start of this version reads every transcript into the statistics store, and on a busy machine that made a session's change of state reach the map seconds late, for as long as the reading ran.
  Now the reading stops whenever the city looks again, and no longer holds the daemon up while it waits for the CPU, so a session reads as working within a fraction of a second while the statistics catch up behind it.

- **No district is called "." any more, and two folders with one name can be told apart.**
  A session whose record names no folder is no longer put in a district of its own on the map; it still counts in the power plant's figures, as "folder unknown".
  Districts whose folders share a name are labelled with as much of their path as tells them apart, such as `github/botropolis` and `botropolis-city/botropolis`, on the map, the City Plan, the sidebar, cards, the journal, the search and the power plant.

- **The sidebar scrolls, so every project and session in it can be reached.**
  With more rows than fit, it showed the first screenful and nothing said the rest, or the hidden projects and their *show* buttons, were there.
  Now the wheel over it, and PageUp, PageDown, Home and End while it is open, scroll it by whole rows, with "N more above" and "N more below" saying what is out of view; the arrows still pan the map.
  When the selection changes, from the map, `tab` or the finder, the list scrolls just far enough to show its row.
  The sidebar also ends above the footer's needs-you line instead of running under it.

- **In a short window the selection's card leaves the sidebar room for its rows.**
  The card docked at the sidebar's foot could take the whole panel, leaving the list no rows at all.
  The list now always keeps four rows, with the selection among them.
  When the whole card does not fit under it, the card stands beside the sidebar;
  where that would cover the lens panel or the stream, the card shows its title and buttons only, and `i` still brings back the full card.

- **Esc closes the sidebar and brings back the hidden interface before it opens settings.**
  With the sidebar open, Esc opened settings on top of it; with the interface hidden by `h`, Esc opened settings unseen, so the keys that followed went to a panel nobody could see.
  Esc now closes the sidebar, and with the interface hidden it shows it again and does nothing else; settings is still what Esc opens when nothing else is.

- **With the interface hidden, `s`, `t` and `x` show it and open their panel.**
  They used to open settings, the timeline or the power breakdown behind the hidden interface, where nobody could see them, and the unseen panel took the keys that followed; clicking the power plant did the same.
  Now any of them brings the interface back with the panel open.

- **The trains are the size of the cars' world, and their wagons no longer pile onto each other.**
  A train was a third as long as it should be beside the cars, and each wagon was drawn two thirds over the one behind it, so a six-wagon train looked like two.
  Trains are now drawn two fifths larger and each runs its own length behind the one in front, on rails set under its body, with trains a wagon and a half apart round the loop.
  A message's carriage along a group's line keeps its old size, so it does not cover the badges it passes.

- **The bridge off the island goes over the railway, and plainly stands on piers out over the water.**
  The road to the bridge crossed the freight loop on the level, so a train and the merge truck could share a spot with nothing to say which passed over the other, and where it left the island it looked like a road running into the sea.
  The road now climbs a ramp at the end of its avenue onto a raised deck, with railings, girders and piers, which crosses over the railway and stays up out to sea.
  Trains pass underneath and the truck drives over the top, and the deck is high enough to clear the tallest wagon.

- **`botropolis-demo stage` works on macOS again, and a headless run there clears up after a dead one.**
  Both tell a process from a later one given the same pid by when it started, which they read only where the system has `/proc`.
  On macOS, staging refused every stand-in, and a private state directory whose pid had been handed on was kept for good.
  They now ask macOS's kernel for the same fact.

- **A session that changes while the daemon starts is no longer missed.**
  The daemon read the sessions and then began watching for changes, so one that changed in between was shown as it had been until it next changed.
  It now reads them again once it is watching, before it serves the city.

- **The key list fits the window, or scrolls.**
  The reference's keys entry had grown to about forty rows and ran off the top and bottom of most windows, so some keys could not be read at all.
  It now takes as few columns as fit, read down and then across, and wraps a long action rather than running past the panel's edge.
  Where no number of columns fits, it is one column that scrolls by whole rows with the wheel and the up and down arrows, saying how many more are above and below.
  It keeps clear of the strip, the footer, the stream and the minimap, working round them rather than shrinking, and a key much longer than the rest wraps in its chip so its column's actions stay close to their keys.
  The wheel no longer zooms the map behind the reference.

- **A signal arriving as the city starts no longer ends it.**
  On Linux the city opens every input device as it starts, and a signal arriving while another program held one of them ended it with "gamepad: Open failed: interrupted system call".
  The thread that runs the city's window no longer takes asynchronous signals, which go to the city's other threads instead.

- **Headless cities started at the same moment no longer lose their display.**
  `--headless` used `xvfb-run -a`, which looks for a free display and then takes it, so cities started together could take the same one, and one of them died with "X connection broken"; eight started at once lost about half.
  It now starts Xvfb itself, which reports the display it took, behind a cookie only that city holds.
  `--headless` needs Xvfb rather than `xvfb-run`: `xorg-server-xvfb` on Arch, `xvfb` on Debian and Ubuntu, `xorg-x11-server-Xvfb` on Fedora.

- **Labels on the map no longer cover one another, and `z` turns them all off.**
  Group badges, a line's station label, district signs and the plates that stand in for them, a first sighting's caption and a road's traffic tag are laid out together each frame: a district sign never moves, and any other label that would cover one steps down or aside, further out on a short leader if it must, and is left off where there is no room.
  Labels are drawn over the world, so a building or a sign no longer hides a badge.
  `z` (`labels_key`) turns every label off and on, as does the settings panel's new `labels` row (shown or hidden); the signs on their posts and the names painted on buildings stay, because they are part of the map, and holding `e` still captions everything.
  Pointing at a car fades every label outside its two districts and its road, with the rest of the city.

- **A city that is not your own leaves nothing behind when it stops.**
  A `--home`, `--headless`, `--record` or `--script` run works in a temporary state directory, and it was sometimes left behind: always on Ctrl-C, and often under load when the run ended normally.
  Ctrl-C or SIGTERM now stops the city and removes the directory; a second Ctrl-C while it stops exits at once and says so.
  A `--headless` run cleans up after itself however its virtual display ends, and the next such run removes any directory left by one that crashed.

- **The city no longer sometimes crashes as it closes on software GL.**
  On Mesa's llvmpipe, as under a virtual display, about one city in thirty exited with a segfault as it shut down: the GL library was unloaded while Mesa's threads were still drawing the last frame.
  The city now keeps the GL libraries it uses loaded until it exits.

- **A recorded clip plays at the city's own speed.**
  With `--record`, the city's clock counted updates while the clip counted drawn frames, and on software GL several updates ran per frame, so clips ran about twice to three times fast and every debounce, ease and transition in them with it.
  A recording now runs exactly one update per drawn frame, so N seconds of clip are N seconds on the city's clock, and it ends by printing how many frames it wrote and how much of the city's clock they span.

- **A recording or screenshot no longer hovers whatever lies under the hidden cursor.**
  With `--record` or `--screenshot` and no pointer shown by the script or set by `--hover`, the city read the system cursor, which a headless display leaves in the middle of the screen, so a district panned to the centre stayed hovered and the rest of the city receded for the whole clip.
  Until the script shows a pointer, it is now over nothing.

- The isometric map's fit kept the stream's corner panel clear only when it was reserved down the right; reserved along the top, the city and the City Plan could run under it.

- **A frame or a run pointed at another home never touches your own city's state.**
  `botropolis city` with `--headless`, `--record`, `--script` or `--home` kept its layout, groups and ledger in your state directory, so every frame rendered from a test home added its districts to your plan.
  Such a run now works in a temporary state directory of its own, removed when it exits, and starts no usage probe or summary.
  With `--home`, it starts from a copy of that home's own `.local/state/botropolis`, so a fixture can carry a layout, groups or a ledger without being written to.

- **The power plant counts all of a session that stopped in the last day.**
  A stopped session's transcript was read through a 64 KB head and tail, so the plant's 24-hour tokens and cost left out most of a long session that stopped yesterday; on one home it showed a third of the fresh tokens and a sixth of the cached ones.
  A transcript active in the last day is now read from the start of the plant's window to its end, and older ones are still read through the window.

- **The map sees every file a session touches in another project and every message it sends.**
  Most of what Claude does in one turn was skipped when the city read a session, so roads, cars and drones between projects, team vans, and the `Depends-On:` lines and pull requests that order a group's line missed most of it.
  Tokens and MCP and skill counts are unchanged; they were already counted once per reply.

## [0.1.7] - 2026-10-15

### Added

- **Shell completion for bash, zsh and fish**, installed by every package where each shell looks for it:
  the `.deb`, `.rpm` and Arch packages, the tarball's `completions/`, and Homebrew.
  Session commands complete what they can act on, with each session's title:
  job ids for `attach` and `stop` (live background sessions) and `rm` (parked ones too),
  session ids for `resume` (parked sessions), and directories only for `new`.
  bash completion needs the bash-completion package, as every bash completion does.

## [0.1.6] - 2026-10-15

### Added

- **macOS, on Apple silicon and on Intel, from a Homebrew tap at pkg.botropolis.city.**
  `brew tap botropolis-city/botropolis https://pkg.botropolis.city/brew`, then `brew install botropolis-city/botropolis/botropolis`;
  `brew services start botropolis` runs the daemon.
  The formula builds from the release's source, and every release is install-tested on both kinds of Mac before the tap is published.
  Nobody has yet looked at the window on a Mac, so tell us what you see.
- **Shell integration for zsh and fish, as well as bash**, from `botropolis shell-init <shell>`:
  `eval "$(botropolis shell-init zsh)"` in `~/.zshrc`, the same with `bash` in `~/.bashrc`,
  or `botropolis shell-init fish | source` in fish's `config.fish`.
  It names no install path, so the line is the same from every package and on every system.
- On macOS, reveal uses `open`, copy uses `pbcopy`, attach falls back to Terminal.app,
  notifications go through AppleScript, and `botropolis doctor` knows it is on a Mac.
- Every release carries its packages a second time under names without the version,
  so `https://github.com/Botropolis-City/botropolis/releases/latest/download/botropolis_amd64.deb` (and `.x86_64.rpm`, the tarballs and the Arch packages) is always the newest release.
  The install commands in the README and on botropolis.city use them.
- **APT and RPM repositories at pkg.botropolis.city.**
  Add the repository once and `apt upgrade` or `dnf upgrade` brings each new release.
  Both are signed; every release publishes to them as part of the release.
- `usage_refresh` sets how often the city fetches plan usage on its own: `manual`, or an interval such as `30m`.
  It is also a row in the settings panel, offering manual, 5m, 15m, 30m and 1h.
  The fetch spends no tokens but is a network call, so the default is `manual`.

### Changed

- `/usr/share/botropolis/botropolis.bash` now loads `botropolis shell-init bash`,
  so `~/.bashrc` files that source it keep working and get the same script.
- With `usage_refresh` at `manual`, the usage boats are off the river until `u` is pressed.
  They then sail in to their readings, stay a minute, and sail off.
  Pressing `u` again while they are there starts the minute over,
  and pressing it while they are sailing off sends them on round to come back in rather than turning them back.
- The usage boats drive to a new reading instead of jumping to it, and only ever forward.
  A reading that falls, such as a window turning over, sends the boat on past the limit and round from the start.

### Fixed

- Choosing a terminal in the settings panel now launches it the way it expects:
  picking alacritty, for example, used to run it without `-e`, so attach failed.

## [0.1.5] - 2026-10-10

### Changed

- **Botropolis has moved to [github.com/Botropolis-City/botropolis](https://github.com/Botropolis-City/botropolis), and has a website, [botropolis.city](https://botropolis.city).**
  The Go module is now `botropolis.city/botropolis`, so `go install botropolis.city/botropolis/cmd/botropolis@latest` works,
  and the module path stays put if the repository ever moves again.
  Links to the old repository redirect.
- The package homepage and the systemd units' documentation link point at botropolis.city.

### Added

- `--script FILE`, with `--record`, plays a choreography while the clip records:
  a pointer that glides to a session, a district or the power plant, clicks, zooms and pans, drawn into the frames.
  It is what films the website's clips.

### Fixed

- The usage boats follow Claude Code's figures while the city is open.
  They were read once at startup, so the river kept whatever the reading was when the city was opened;
  the city now re-reads the cache whenever Claude Code updates it.
- Recorded clips play at the speed the city runs.
  On a slow renderer every recorded frame advanced the animations three or four frames' worth, so clips ran fast and shook;
  while recording, time now advances by the frame rather than by the clock.
- Recording no longer stalls the city: frames are encoded off the game loop, and a frame on disk is always whole.

## [0.1.4] - 2026-10-09

### Added

- `--window WIDTHxHEIGHT` sizes the city's window, and under `--headless` the virtual display with it.
  `--fps` picks how many frames a second `--record` writes, and refuses a rate that does not divide 30.
  Both default to what they were, 1100x760 and ten.

### Fixed

- A plain `claude` from the shell integration no longer fails in a directory nobody has trusted yet.
  `claude --bg` cannot show the trust prompt,
  and the advice it printed — run `claude` there once — went straight back through the same wrapper.
  `botropolis new` now exits 3 for that refusal, and the wrapper runs that one session in the foreground so the prompt can appear.
  Later sessions in the directory go to the background as before.
- The usage boats and **generate summary** read from `--home` rather than the real home.
  A city pointed at other data showed the real account's limits,
  and a session sharing an id with a real one would summarise the real one.

## [0.1.3] - 2026-10-06

### Added

- **A summary on the building card.**
  The card's new **summary** button, or `i`, opens the recap Claude Code writes into a transcript when a session is left waiting.
  It is shown as written, under a line giving its age and how many turns the session has taken since.
  A session with no recap falls back to its last prompt and offers **generate summary**.
  That button runs your own `claude` CLI, so it uses your existing login and needs no API key.
  It runs Haiku with no tools, no MCP servers and no user settings, writes no transcript, and is fenced out of the city like the usage probe, so generating a summary never adds a building.
  `status --json` carries each session's `recap` and `lastPrompt`.

## [0.1.2] - 2026-09-30

### Added

- **aarch64 packages.** Every release now carries `.deb`, `.rpm`, Arch and tarball artefacts for both `amd64` and `arm64`.
  The Arch package had declared `aarch64` support since it was written and nothing had ever built it.

### Changed

- The release tarball now carries `packaging/` — both systemd user units, the desktop entry, the icon and the shell integration — so a package built from the tarball installs the same set as one built from source.
- Binaries are built with `CGO_ENABLED=0`.
  Ebitengine reaches GL and X11 through `dlopen`, so nothing needed cgo, and dropping it is what makes building for another architecture a plain cross-compile with no toolchain.

## [0.1.1] - 2026-09-30

### Fixed

- The `.deb` and `.rpm` packages no longer require `libXxf86vm` or ALSA.
  Neither is referenced by any binary — they came from the build headers CI installs, which is not the same as a runtime need, and requiring ALSA made every user install a sound stack for a program that never opens one.
- The packages now declare `hicolor-icon-theme`, which they need for the icon they install into that tree.

## [0.1.0] - 2026-09-30

First release.
Botropolis draws every Claude Code session on the machine as a city, and starts, opens, parks and wakes them through the `claude` CLI.

### Added

- `botropolis` — the isometric city.
  One district per project, one building per session, and every object stands for one datum you can read by hovering it.
  A building's height is its context used and its beacon is its state;
  rovers, drones, flags and smoke stand for turns in flight, subagents, pull requests and API errors.
- The plaza landmarks — the power plant for API spend, broken down by model, project and session over an hour, a day or a week;
  radio towers for MCP servers;
  the library for skills;
  and the city hall for Claude Code's own rollup.
- The river, the freight loop and the avenues — arrivals and departures, the day's tokens as one train per model, and traffic between projects whose sessions message each other or edit each other's files.
- Nine info views, a sidebar, a breakdown panel, a searchable list and a timeline that survives the window closing,
  including a "while you were away" summary of what needed you since the city was last seen.
- `botropolisd` — a daemon watching `~/.claude` by inotify, taking hook pushes, and serving the city model over a unix socket.
- `botropolis-hook` — the hook forwarder, installed into `~/.claude/settings.json` by `botropolis install-hooks` with a backup kept.
- Session management wrapping the `claude` CLI: `new`, `attach`, `stop`, `resume`, `rm` and `prune`.
  Botropolis never writes to `~/.claude` itself.
- `botropolis status`, `bar` for a status bar such as waybar, `tui` for the terminal table, `events` for the daemon log, and `doctor` for the install.
- `botropolis version`, and `--version` on all three binaries.
- Settings from flags, environment and `~/.config/botropolis/config.{toml,yaml,json}`, with an in-city settings panel that writes the file back.
- Headless rendering — `--screenshot` for one frame, `--record` for a sequence, and `--keys` to press keys first.
- `.deb`, `.rpm` and Arch packages, each installing the three binaries, both systemd user units, the desktop entry, the icon, the shell integration and the licences.

### Security

- The usage probe runs inside a fenced directory the session loader skips, and prunes its own transcripts, so reading usage cannot make Botropolis see itself as a session.

[Unreleased]: https://github.com/Botropolis-City/botropolis/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/Botropolis-City/botropolis/compare/v0.2.3...v0.3.0
[0.2.3]: https://github.com/Botropolis-City/botropolis/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/Botropolis-City/botropolis/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/Botropolis-City/botropolis/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Botropolis-City/botropolis/compare/v0.1.7...v0.2.0
[0.1.7]: https://github.com/Botropolis-City/botropolis/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/Botropolis-City/botropolis/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/Botropolis-City/botropolis/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/Botropolis-City/botropolis/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/Botropolis-City/botropolis/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/Botropolis-City/botropolis/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Botropolis-City/botropolis/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Botropolis-City/botropolis/releases/tag/v0.1.0
