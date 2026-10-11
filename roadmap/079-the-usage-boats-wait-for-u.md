# 79. The usage boats are always out of date, because only `u` ever refreshes them

**Done 2026-10-10 (`6918980`, r390). Filed 2026-10-10 (r378) at Aria's request. Her words: "They are always way out of date, and I am always manually hitting u. I thought we were getting live data since it's local. Only remote calls should be limited." Aria's call on the interval; the rest follows from her rule.**

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

## Measured, and what was built (2026-10-10)

**The probe spends no tokens.**
`claude -p "/usage" --output-format json` reported zero input and output tokens, `total_cost_usd` 0, `duration_api_ms` 0, no turns and an empty `modelUsage`.
The surviving probe transcript agrees: two user records for the local command, its stdout, and a `cost-state` of zero, with no assistant turn.
**It is a network call.** `cachedUsageUtilization.fetchedAtMs` in `~/.claude.json` moved to the moment of the run, and the run took 2.2 s wall-clock.

Aria's rule for that case was an interval and a manual-only setting, and then that manual should be the default and the interval customisable:

- **`usage_refresh`** is `manual` (the default) or an interval, from the flag, the config file, or a settings row offering manual, 5m, 15m, 30m and 1h; anything else in the file refuses to start.
  On an interval, a probe goes out when both the reading and the last probe are older than it (`usageDue`, `pkg/render/gauges.go`); `usageDebounce` and the in-flight guard still apply.
  An automatic probe sends no courier and sets no status line. It never runs while recording or taking a still.
- **By hand the boats are away until called.** Aria: "we should have the boats not there by default. We should have them drive up when you update manually and then drive off the screen."
  When the probe that `u` set going comes back, they sail in from below the run, hold for `GaugeStayFor` (a minute), and sail off past the limit. A press inside the debounce still calls them up with the figures already fetched.
  Switching to manual sails the boats off; switching away from it brings them in.
- **The boats drive instead of teleporting.** Aria: "when things change, the boats should drive forward instead of teleporting."
  Every boat has a track (`pkg/city/gaugeboats.go`) and only moves forward along it, at a run per `GaugeRunFor` (3 s) and never faster than `GaugeDriveMin` (1 s) for a short move. A reading that falls carries on off the top and comes round from the bottom.
  The first reading places the boats rather than sailing them in, so stills and films are unchanged; reduced motion skips the travel.
- **Stills and films show the boats on station whatever the setting**, since nothing can call them up in a frame.

Not built from the list above: probing on focus or after a `Stop` hook, backing off after failures, pausing while minimised, and a stale look for a failing probe.
None is needed while the default is manual; they become worth having if a short interval turns out to be the setting in use.

## Closed

Done in `6918980` (r390), with `a2edff8` (r391) pinning the one visit case it left untested, and closed by Aria the same day.
It was built to her later rulings rather than to the proposal above: the default is manual, the interval is hers to set, and the boats visit and drive rather than standing on the river and jumping.
The tests are `pkg/city/gaugeboats_test.go`, `pkg/render/usageschedule_test.go`, `TestGaugeTrack`, `TestUsageRefreshKey`, `TestParseUsageRefresh` and `pkg/app/settings_test.go`; the forward-only rule was mutated red, and fails two of them.

**Flagged for Aria:** there is no frame.
A still or a film shows the boats on station whatever the setting, so neither can show a visit or a drive; the motion has been checked by tests only, and the live city is where it gets judged.
And the unbuilt half of the original list stands as written above — if a short interval becomes the setting in use, that is a new item rather than a reason to reopen this one.
