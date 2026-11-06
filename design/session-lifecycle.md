## Session lifecycle

| State | Derived from | On the map | Click |
| --- | --- | --- | --- |
| **Needs you** | pid alive and the last main-thread message handed the turn back, or a `Notification` hook | the building's light pulses | terminal running `claude attach` |
| **Working** | pid alive, mid-turn (`PreToolUse` seen, or last message called a tool) | lit windows, worker at the bench, cranes for subagents | attach |
| **Waiting** | the turn was handed back after the session armed its own watch — `ScheduleWakeup` (not stopped), `Monitor`, `CronCreate` — with nothing in flight, so it will carry on by itself; you can talk to it, it does not need you (added 2026-09-18 at Aria's request) | teal beacon, lit | attach |
| **Unattended** | `kind: background` with no terminal attached (no client on the job's pty socket in `daemon/roster.json`) | night-shift lamp, alarm clock | attach |
| **Empty** | pid alive and nothing typed yet: no transcript, or one with no prompt and no reply (added 2026-09-18, audit bug 2) | a vacant plot, no building, never counted | attach; `prune` clears it once it has sat for an hour |
| **Parked** | pid gone; `SessionEnd` seen or inferred; transcript resumable | boarded-up, grey | `claude --bg --resume <id>` then attach |
| **Gone** | transcript aged out, or you demolished it (`claude rm`) | nothing | — |

"Done" uses both signals: the `SessionEnd` hook when it arrives, inference (pid gone + turn handed back) when it does not.

**Working to needs you is held for three seconds** (`daemon.IdleHold`, item 346).
A session whose turn ends keeps showing as it was, tool and all, and needs you only if nothing new has happened by then,
so a pause of a second or two between turns is never seen: not on the map, not by `status`, `bar` or `notify`, and not in the event log, which all read the daemon's view after the hold.
Working again cancels the hold.
A permission prompt, a question, a session ending or resting, and a session that was not working all show at once, because a delay there is the worse defect.
