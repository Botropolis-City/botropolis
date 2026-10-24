# Usage profile

Measured with `tools/analyze-history.py` over a 30-day transcript retention window on the machine botropolis was built for.
It is what the design in [DESIGN.md](../DESIGN.md) is shaped around.

**Aggregates only, deliberately.**
The generator also prints a per-project table, the longest sessions by title, and the last prompt of each session.
Those describe an employer's repositories, tickets and work rather than this program's design, and they are not in this file or in this repository's history.
The design was only ever shaped by the shapes below.
Running the tool locally reproduces the full output for whoever runs it, which is where it belongs.

## Headlines

- **CLI only**: 152 of 153 sessions started from the terminal, so desktop-app focus state is never available.
- **Bimodal**: median 9 minutes active, p90 nearly 4 hours; 16 sessions spanned more than a day of wall time.
- **Concurrent**: two or more sessions active 29% of the time, peak 8.
- **Context is the scarce resource**: median 166K at last request, p90 400K, max 813K, 11 compactions ever.
- **Subagents are a quarter of spend**: 10.6M output tokens across 1,226 transcripts.
- **Cache dominates the wire**: 98% hit rate.

## Age of last activity — is it "done"?

    >30d: 1    7-30d: 89    3-7d: 8    1-3d: 33    <1d: 30

The long tail is why "done" cannot be inferred from a transcript existing, and why a stopped session needs its own state: resting, then shuttered and overgrown as it stays quiet.

## How sessions ended, by last main-thread record

    assistant_done 144    (none) 10    assistant_tool 6    user 1

Ten sessions with no terminal record at all is the case the pid-gone-plus-turn-handed-back rule exists for.

## Duration: wall against active

    wall    median 0h09m   p90 19h28m   max 504h36m
    active  median 0h08m   p90  3h44m   total 171h39m
    spanning more than a day of wall time: 15 (resumed, or left open)

Wall time is not work, which is why the map ages a session by activity rather than by age.

## Activity by hour, and by weekday

    00:623  01:461  02:126  03:687  04:1096 05:1204 06:119  07:0
    08:17   09:1478 10:6635 11:9062 12:3487 13:4963 14:10501 15:9576

    Mon 14561   Tue 20947   Wed 21283   Thu 5346   Fri 6663

The 03:00–05:00 band is scheduled loops rather than anyone awake, which is why unattended work needed a state of its own.

## Concurrency: simultaneous active sessions, 5-minute buckets

    1: 908   2: 233   3: 74   4: 36   5: 19   6: 8   7: 3   8: 3

A single pane of glass has to hold eight at once, and usually holds one.

## Models and effort

    effort: high on every one of 27,597 turns

Effort never varies on this machine, so it is not worth drawing.
The model mix spans four families inside one retention window, which is why the freight loop carries one train per model seen rather than a fixed set.

## Subagents, compaction, MCP

    subagent transcripts: 1,226
    compactions, lifetime: 11
    sessions using MCP: 32 of 153, across six servers

Subagents at a quarter of output tokens are why they are drawn at all.
Eleven compactions in a lifetime is why context is shown as a proportion of a window rather than as a countdown.
