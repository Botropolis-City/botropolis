## What it costs

Roadmap phase 18, 2026-09-22.
The daemon was always within its bar; the renderer was not, and redrew the whole city thirty times a second whether or not anything had changed or anyone was looking.

Three things fixed that, cheapest first.
A window nobody can see draws nothing and waits in `Draw` rather than trusting the display's throttle, because a hidden window has no vertical blank left to wait for and a window manager is free to leave one focused while it is unmapped.
The city under the traffic — ground, avenues, beams, district floors, the plaza, the camp ties — composes into an offscreen image keyed on `(generation, camera, heading, view, size, night, labels, hover)` and blits until that key changes.
And a frame whose tick has already been painted is skipped, with the tick itself dropping to ten when `city.Scene.Animating` says nothing on the map moves.

**What the draw order costs** (bug 42, 2026-09-26). Ordering the city back to front used to be one sort on one number per drawable. A number cannot order a point against a footprint — a building spans a range of depths and was being compared as though it were a point at one corner — so the sort is now a first pass and a pairwise predicate corrects it, over only those pairs whose depth intervals and screen columns both overlap. On the agent's rig, 508 drawables: 95.9 µs before, 174.0 µs after, **+78 µs a frame**, or 0.23% of one core at 30 fps. Allocations went from 3 to 1131, which is the edge lists and is the obvious thing to pool if this ever matters.

**Numbers are a measurement of one desk, so the desk is written beside them.**
CPU is `utime+stime` from `/proc/<pid>/stat` over a twenty-second sample as a percentage of one core, taken by `tools/measure-render` on a display that already answers.
Every row carries the frames drawn during the sample, because a cheap app and a stopped one look alike without it.

**The memory column is `VmRSS`, and `VmRSS` is the wrong number.**
It is kept because that is what was measured, and a row relabelled after the fact is a row that has stopped being evidence — but nothing new should be quoted in it, and no two rows in it should be compared unless they ran against the same graphics driver.
Item 61, 2026-09-26: of a client sitting at 357.3 MB `VmRSS`, **68% is shared libraries under `/usr/lib`** — `libnvidia-gpucomp`, `libgallium`, `libnvidia-eglcore`, and `libLLVM` when there is no hardware to avoid it — file-backed, clean, and shared with every other process on the machine that draws anything.
A third of this table is therefore a measurement of a Mesa release.
The city's own memory is `Private_Dirty`, which was **85.7 MB** on the virtual display and **75.4–86.5 MB** on the desk, against `Pss` of 188–254 MB.

**An earlier version of this paragraph rested the correction on two rigs agreeing to within 5% on `Private_Dirty`. That agreement was not real and has been withdrawn.**
The virtual-display figure behind it was sampled from a `--record` run, which accumulates frames and whose memory climbs the longer it runs; the same instrument gives 86 MB, 823 MB and 1.35 GB depending on when it is read and which GL path it lands on.
It was not measuring a steady state and should never have been set beside a desk figure.
Two numbers matching is the most persuasive and least reliable evidence available, because nothing about a coincidence announces itself — and it is worth recording that the retraction cost more than the claim was ever worth.

**What the ruling actually stands on is one rig, broken down by mapping, which does not need a second rig to agree.**
On the desk, drawing: `VmRSS` 388–390 MB against `Private_Dirty` 114.6–116.5, with about **147 MB of NVIDIA and Mesa** that is file-backed, clean, and shared with every GL process on the machine.
That is an argument about what the pages *are*. A page of `libgallium` is not the city's memory whatever either rig reports, and `VmRSS` counts it whatever the window is doing.
So: quote `Private_Dirty`, or `Pss` when a shared page genuinely is a cost, and name whether the window was drawing — a window that exists holds about 77 MB and the same window drawing holds about 115, so a figure without that state attached is missing a third of itself.
Bugs 22, 27 and 28 quote `VmRSS` and their absolute figures should be read as that and not as the city's footprint.

| Rig | Build | Visible | Hidden | RSS (`VmRSS`, see above) | Peak |
| --- | --- | --- | --- | --- | --- |
| Aria's desk | r156, before phase 18 | 56.7% | 57.7% | 244 MB | 740 MB |
| Aria's desk | items 1 and 2, first cut | 35.6% | 50.3% | 268 MB | — |
| Aria's desk | item 1 redone | 40.7–46.1% | 0.8%, 0 frames | 247 MB | — |
| Agent's rig | r156, before phase 18 | — | — | — | — |
| Agent's rig | item 1 redone | 28.7%, 1200 frames | 0.6%, 0 frames | 231 MB | — |
| Agent's rig | item 3, frames capped | 20.3%, 600 frames | — | 235 MB | — |
| Agent's rig | item 3 complete | 18.8%, 600 frames | 0.2%, 0 frames | 208–236 MB | 536 MB |
| Aria's desk | item 3 complete | 14.5%, 360 frames | 0.3–0.4%, 0 frames | 212–234 MB | 504 MB |

Aria's desk: 1920×1200, the window 636×1120 under i3's tiling, 8 live sessions and 72 in all, 40–45-second warm-up.
The agent's rig: the same machine's `DISPLAY=:0` through a separate daemon, window at Ebitengine's default, 15-second warm-up.
The two disagree on the visible figure by a third on the same commit — different cities on different glass — and agree on the hidden one to a tenth of a percent.

The frame counts are worth reading as a number in their own right.
Thirty a second is the tick; eighteen means `city.Scene.Animating` found nothing moving for two frames in five, so the tick had dropped to ten for that share of the sample.
It gates off far more often on a real desk than the agent's own city suggested, which is to say the tick drop is worth more than the measurement that introduced it credited it with.
A city with work in it always has something moving; a city mostly parked does not, and that is the common case.

RSS is given as a range because it is not a property of the build.
An idle app allocates too little to make the collector run, so memory is handed back explicitly when the window goes quiet — but whether that has happened yet by the time a sample is taken is timing.
One rig saw 236 MB fall to 208 when hidden; the other saw 212 rise to 219 and 224 to 234 across runs.
A virtual display cannot stand in for either *on CPU*: software rasterising turns everything into fill rate, where one full-screen blit shades as many pixels as the sprites it replaces, and no draw-call saving is visible at all.
It does not stand in for memory either, though the reason is different and took a retraction to find: a virtual display may fall back to llvmpipe, whose buffers are nothing like a driver's, and the same command on the same machine measured 357 MB one way and 1.35 GB the other.
An earlier draft here claimed the opposite on the strength of one coincidental match.
`BOTROPOLIS_FRAMETIME=1` reports what a frame costs the CPU and how often the static layer was reused; that is the hardware-independent signal, and it predicted item 2's cut on real glass to within a percent (3.69 ms to 2.30, −38%, against −37% measured).
A gate that is hard to exercise by hand is itself worth recording: i3 will not hand focus to the window from a non-interactive shell, so an unfocused window correctly draws nothing and a naive sample reads the hidden figure whatever it meant to measure.
That is why every row carries its frame count.

Memory: 151 MB of the settled RSS is the atlas on the card — nine 2048-pixel pages — and cannot go without loading it again.
The start-up peak is `assets.LoadKits` decoding all nine pages before uploading any, so both copies are alive at once; a page at a time would fix the peak itself.
An idle app allocates too little to make the collector run, so memory is handed back explicitly when the window goes quiet and after the atlas upload.

### How to measure this, and what does not work

Two instruments, and the second one only because the first cannot be driven from a script on this rig.

**Whole-process CPU** — `utime+stime` from `/proc/<pid>/stat` over a sample, `VmRSS` from `/proc/<pid>/status` — is the number that matters, and it has to be taken with the window open on a real display.
`tools/measure-render` does it, and `--place` implements the i3 recipe: name the focused workspace explicitly, because a window launched from a non-interactive shell lands on whatever workspace i3 last used; float it so the tiling is not disturbed; move the pointer into it, because focus_follows_mouse snaps focus back to wherever the pointer sits; then focus.

**That recipe is not sufficient on this machine, and it took a while to establish why.**
A city window placed exactly that way — X input focus confirmed on it, floating, mapped, pointer inside — draws *zero* frames and costs 0.3% of a core, indefinitely.
A minimal Ebitengine program placed identically reports `focused=true visible=true minimised=false`, so the gate in `watching()` is not the thing refusing.
Neither the frame counter nor the gate's own diagnostic ever prints, which means `Draw` is not being called at all.
So the honest position is that whole-process CPU for a *non-capturing* window cannot be scripted here; it has to be taken by hand, or by whoever's desktop answers differently.

**Frame time** — `BOTROPOLIS_FRAMETIME=1`, which prints ms of drawing every 120 frames — works whenever the run is capturing, because `capturing()` short-circuits the gate.
It is also the hardware-independent number: it counts the work of issuing the drawing rather than the fill rate a software display would charge for.
The catch is that `--record` costs about three seconds a frame, and none of that is drawing: `frame(screen)` reads the framebuffer back off the card, which stalls the pipeline.
That cost lands outside the timer, so the ms/frame figure is clean, but a twelve-second recording takes six minutes of wall clock.

**The work a frame asks of the GPU** — draw commands, their indices, and render-target switches — is the third, and the one that gates (item 194).
`TestTheWorkAFrameTakes` builds the city with `-tags ebitenginedebug`, records a second of the metropolis demo headless by day and by night, and counts each frame from Ebitengine's own command log (`testing/helpers/framelog.go`).
Counts do not change with load or driver, so a regression in drawing fails it on any machine.
A driven city cannot count this, because it drops every draw before Ebitengine's queue sees it.
Indices are the count that moved when the heap grew (item 156): anti-aliased vector fills multiplied them 6.6× with the commands unchanged.
Each count has a target from the 10 ms budget, which skips while unmet and fails once met, and a ceiling under it that gates and only goes down.

**A building's look is baked, and only a still one** (item 192).
Each run of a building's steps that does not move is drawn once into an image of its own and blitted after; the cache holds a sixteenth of the city's settled memory budget, least recently used out first.
A look is baked only once its key has held for a frame, so a zoom draws buildings straight, as before, and the frame after it stops bakes them all at once: on the metropolis, about 430 commands and 60 switches for one frame.
Looks are baked onto pages of their own (item 193): as images of their own on Ebitengine's shared atlas, each was moved onto it some ten frames later, a copy and two target switches in a frame where nothing changed, and they batched only once they had.
Panning by whole pixels keeps every look, since the key holds the foot's phase within a pixel, not its place.
`TestTheLooksAFrameKeeps` gates that a settled frame bakes nothing, at fit and up close, and reports a wheel zoom and a held arrow without gating them.
`botropolis-demo sheet --check-looks` holds every look to the building drawn directly, from every heading, by day and by night.

**A guard clause can be false for every case the new code was written to handle.**
Making the five moving things hoverable was finished, tested and would have done nothing: `hoverSprites` consulted the sprite hits only when the world hover was open ground, and `Hit.ground()` is false whenever a building is under the pointer.
A rover stands at a door, a drone circles a roof, a flag stands on one and smoke rises off it — all four are inside or above the footprint of the building they belong to, so the guard excluded every case the change existed for.
It was invisible to the tests, which exercised the new code directly and passed, and invisible to a frame, which would have shown four cards that never appear and sent the search to `noteHit`, where nothing was wrong.
It was visible only by reading the path the call actually takes.
**When adding a case to a function, check what the existing guards say about that case — not only that the case is handled once it is inside one.**

**The instrument must not be inside the thing it measures.**
That has now cost this project three times: a focus gate measured under Xvfb, where nothing is ever focused, so the app idled and the number recorded an app that was not running; a zero-gap measurement of the plant that could not tell *resting on* from *occluded by*, because the plaza is drawn over whatever it covers; and a `pgrep -f "make sprites"` waiter whose own command line contains the string `make sprites`, so it matched itself and could never exit.
The last one has a rule worth stating flatly: **a waiter must never match on a string its own command line contains.**
`pgrep -x`, a pidfile, or `wait` on the job are immune; `pgrep -f "<the thing I am also called>"` never is.

**A run that draws nothing still reports a plausible CPU figure**, which is the trap worth naming: a quarter of a core, spent sleeping, looks exactly like a cheap frame.
Three of eight runs on one desk did that.
So there are two diagnostics behind `BOTROPOLIS_FRAMETIME`, because there are two ways to draw nothing: `QUIET no frames` names which of focus, visibility and minimisation failed, and `QUIET update is running but Draw has painted nothing` catches the harder case above, where the gate never gets a word in.
Every measurement in the table below carries its frame count for the same reason.

### The standing city is composed once (item 193)

Every drawable of the frame runs as it always has, onto a stand-in image that is never drawn: each draw is recorded instead, with the box it would ink and a hash of what it would draw (`pkg/render/ink.go`).
Everything a drawable notes as it draws — a hit, a light, a word seen, a look baked — happens there, once, so hit testing needs no replay.
The still drawables are composed over the ground into one image, which a settled frame blits once; a still drawable that would paint differently is composed again inside its old and new boxes only.
What moves — cars, drones and their shadows, trains, boats, tugs, hauls, messages, couriers, inspectors, deliveries, ribbons, sparks, the fountain, a building with a live step — is drawn again inside rectangles merged from its boxes, from the ground up: every drawable whose box meets one, still or moving, played clipped to it in the frame's order (`pkg/render/stillframe.go`).
Inside a rectangle those are the same draws in the same order as drawing the whole frame, so nothing blends twice, which is what sank the attempt below.
The night's lights are kept the same way, and the group lines are a layer of their own over the movers.
The ground layer is recorded too, so a snapshot that rebuilds the city without changing how it looks composes nothing.
A key new this frame is drawn straight rather than composed, as a zoom's frames are; it is composed once it holds for a frame.
A pan by whole pixels is the exception, and the most common move: what every draw paints is hashed relative to the camera's origin — a painted mesh by the shapes it was asked for, since its float32 vertices round a hair differently as they move — so a whole-pixel pan changes no hash, and the ground, the still city, its lights and the group lines are shifted, with only the strips the pan uncovers composed.
The ground is compared draw by draw, and where it is painted again so is the city over it.
The rectangles of movers are played as one stream, a draw at a time into every rectangle it meets, and draws from one source are put together wherever they pass no draw they overlap; the night's lights the same, a window's panes only where they land.
Each map label is baked onto a page of labels once it holds, and blitted from the group lines' layer, so the two go down as one.

`botropolis-demo sheet --check-frames` draws every frame of the metropolis both ways — as it stands, with a building selected, with traffic shown and a car hovered, with a view up, by day and by night — and holds them within 3 levels; `TestRawDrawSites` holds every draw in `pkg/render` and `pkg/shapes` to the recorder or a listed reason, since a draw the record does not see goes stale.
On the metropolis at 1600×900 a settled frame went from 131 commands and 34,002 indices to 22 and 4,890 by day, and from 188 to 40 commands and 5,742 indices by night; the group lines alone had been 20,520 indices.
A held arrow's pan went from 138 commands and 54,500 indices a frame to 29 and 5,688.
With the same activity, none, the metropolis costs what the hamlet does: 9 commands each by day and 14 by night, and indices within 5% (Aria, 2026-10-21: "Hold movers equal").
Against the city as it was at r862, from the same staged moment, no pixel differs by more than 3 levels by night or 2 by day (`docs/screenshots/r867-item-193-night-before-after.png`).
The CPU of a driven frame went from 3.05 ms to 2.14 by day and 3.74 to 2.70 by night, with half the allocations, before the queueing it no longer does.
It holds four images the frame's size, the still city, its lights, the group lines and the one a layer is shifted through, and a 1024-pixel page of labels: 26 MiB at 1600×900 and 131 MiB in a 4K window, against the 512 MiB budget.

Two things it found on the way.
A layer the frame's size must have its own texture: on Ebitengine's shared atlas, beside the sprites the lights are cut with, the same cuts laid onto it left a lit pixel dark that they kept lit on a fresh image, and the ground drawn onto one lost the end pixel of a rail.
And a cut must meet the light a pane spreads past its edge, not only the pane, or where it lands depends on what other windows are laid.

### Why the sorted list was not cached until item 193

Tried and reverted, 2026-09-22 (`2eddc63`, reverted in `ad3fedf`).
The static city is composed once and reused; the sorted list of buildings, landmarks, trees, lamps and movers is not, and that is deliberate.

Caching it means a mover must redraw whatever still thing it passes in front of, or a car is painted over the building it is driving behind.
Redrawing that sprite over the cached layer blends its anti-aliased edge a second time, so a hair of softness spreads across the map.
Three arrangements were measured and all three pay it: the full version changed 29,597 pixels of 3,344,000, the fallback that leaves buildings per-frame changed 19,218, and a version with no transparent layer at all changed 18,403.
The cost is inherent to caching a list something else has to draw over, not a bug to tune out.

What it buys does not cover that.
On the author's rig the full version took 2.7 ms/frame to 1.8; on Aria's desk, with eight live and seventy parked sessions, the same instrument read 2.62 to 2.45 and process CPU moved 19.8% to 19.5% — about two points of a core.
The win scales with how much of a frame is static, and on a real city what remains is movers: cars, trains, drones, workers, sparks.

The conclusion that matters is the one that followed: if what is left is movers, then **hiding networks is the lever**, and the info views of roadmap phase 19 are the performance work rather than a feature resting on it.
