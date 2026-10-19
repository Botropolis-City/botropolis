# Changelog

All notable changes to Botropolis are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/Botropolis-City/botropolis/compare/v0.2.2...HEAD
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
