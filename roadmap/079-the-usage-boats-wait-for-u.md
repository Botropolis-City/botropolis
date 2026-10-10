# 79. The usage boats are always out of date, because only `u` ever refreshes them

**Open. Filed 2026-10-10 (r378) at Aria's request. Her words: "They are always way out of date, and I am always manually hitting u. I thought we were getting live data since it's local. Only remote calls should be limited." Aria's call on the interval; the rest follows from her rule.**

## Why the figures are not local

The boats read plan usage — the five-hour and weekly percentages, and a spend cap where the plan has one.
Those are the account's limits, held by Anthropic and shared by every device on the account; no transcript carries them, and the transcripts cannot be turned into them, because the city knows the tokens it spent and not the limit they are a percentage of.

What *is* local is Claude Code's cached copy, in `~/.claude.json` (item 49).
The city already treats that as free: `followUsage` (`pkg/render/gauges.go`) stats the file as each snapshot lands and re-reads it when it has changed, so the boats move the moment Claude Code fetches.
**The problem is that Claude Code rarely fetches.** Nothing in the city asks it to except `u`, which runs `claude -p "/usage"` in the probe directory (`RefreshUtilization`, `pkg/claude/usage.go`).
So the boats are as fresh as the last time Aria pressed `u`.

That probe *is* the remote call. By Aria's rule it is the one thing that should be limited, not the one thing that should wait to be asked for.

## What it wants

The city refreshes on its own, within limits that belong to the remote call:

- **On a schedule while the city is open:** when the reading is older than an interval (proposed: 10 minutes; Aria's call), probe.
- **When it matters most:** on window focus if the reading is stale, and after a session's turn ends (the `Stop` hook the daemon already receives), since that is when the figures move. Debounced, so a burst of ending turns is one probe.
- **Never more than the floor:** `usageDebounce` (30 s) stays as the floor for every path, and an automatic probe never runs while one is in flight (`mayAskUsage` already guards both).
- **Backing off on failure:** a probe that times out or finds no figures doubles the interval up to an hour, so a machine that is offline does not run the CLI every ten minutes for nothing.
- **Only when it can be seen:** no automatic probe while the window is minimised or the screen is locked; on return, the focus rule catches it up.
- `u` stays, for "now".

## What to keep straight

- **Each probe is a `claude` process and a transcript.** Item 67 prunes the transcripts to the newest, and item 54's fence keeps them out of the city, so neither grows; an automatic probe should still say so in the event log, so a run of failures is visible.
- **The courier boat (item 56) means "you asked".** An automatic refresh is not something anyone asked for, so either it sends no courier or it sends a quieter one — sailing a boat across the river every ten minutes would be the unexplained motion this project exists to remove.
- **Find out whether the probe spends tokens.** `/usage` is a slash command and should not reach a model, but that is a belief, not a measurement. Check the probe transcript for an assistant turn with usage on it before choosing an interval.
- **Not an option:** calling Anthropic's usage endpoint directly would skip the subprocess, but it means reading Claude Code's OAuth credentials, and botropolis reads `~/.claude` for state, never for secrets.
- The age is already on each boat's hover card (`GaugeCard`, "read 2h ago (u to refresh)"), so "out of date" is discoverable but not visible. With refreshes automatic it should rarely matter; if a probe keeps failing, the boats should look stale without being pointed at.
